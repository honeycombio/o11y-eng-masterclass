package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	collectortrace "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	"google.golang.org/grpc"
)

// This is an end-to-end test of the shipped command rather than of a function
// inside it, because the bug it exists to prevent lived precisely in the gap
// between the two.
//
// The BatchSpanProcessor silently drops spans that overflow its queue (2048 by
// default). An earlier version of this seeder generated all 10,000 requests and
// then flushed once at the end, so ~95% of them were dropped — and the command
// exited zero, printing a cheerful success message. Every unit test passed,
// because the scenario package was generating the data correctly. The data just
// never arrived.
//
// So: run the real binary, point it at a real OTLP receiver, and count what
// actually lands on the wire.

// seedCount must produce more spans than maxQueueSize, or this test cannot
// observe a dropped-span regression at all. TestSeedVolumeExceedsQueue enforces
// that rather than leaving it to a comment.
const seedCount = 2500

// wantSpansPerTrace is the root plus its four steps. Both routes have exactly
// four steps, so this is deterministic.
const wantSpansPerTrace = 5

type receivedSpan struct {
	name     string
	parentID string
	attrs    map[string]string
	start    uint64
	end      uint64
}

func (s receivedSpan) durationNanos() uint64 { return s.end - s.start }

type collector struct {
	collectortrace.UnimplementedTraceServiceServer
	mu    sync.Mutex
	spans []receivedSpan
}

func (c *collector) Export(_ context.Context, req *collectortrace.ExportTraceServiceRequest) (*collectortrace.ExportTraceServiceResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, rs := range req.ResourceSpans {
		for _, ss := range rs.ScopeSpans {
			for _, span := range ss.Spans {
				attrs := make(map[string]string, len(span.Attributes))
				for _, kv := range span.Attributes {
					attrs[kv.Key] = renderValue(kv.Value)
				}
				c.spans = append(c.spans, receivedSpan{
					name:     span.Name,
					parentID: fmt.Sprintf("%x", span.ParentSpanId),
					attrs:    attrs,
					start:    span.StartTimeUnixNano,
					end:      span.EndTimeUnixNano,
				})
			}
		}
	}
	return &collectortrace.ExportTraceServiceResponse{}, nil
}

func (c *collector) collected() []receivedSpan {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.spans)
}

func renderValue(v *commonpb.AnyValue) string {
	switch val := v.GetValue().(type) {
	case *commonpb.AnyValue_StringValue:
		return val.StringValue
	case *commonpb.AnyValue_IntValue:
		return fmt.Sprintf("%d", val.IntValue)
	case *commonpb.AnyValue_BoolValue:
		return fmt.Sprintf("%t", val.BoolValue)
	default:
		return v.String()
	}
}

// runSeeder starts an in-process OTLP receiver, runs the seeder against it, and
// returns everything the receiver saw.
func runSeeder(t *testing.T) []receivedSpan {
	t.Helper()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	sink := &collector{}
	server := grpc.NewServer()
	collectortrace.RegisterTraceServiceServer(server, sink)

	served := make(chan struct{})
	go func() {
		defer close(served)
		_ = server.Serve(lis)
	}()
	t.Cleanup(func() {
		server.Stop()
		<-served
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "run", ".")
	cmd.Env = append(os.Environ(),
		"OTEL_EXPORTER_OTLP_ENDPOINT="+lis.Addr().String(),
		fmt.Sprintf("SEED_COUNT=%d", seedCount),
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("seeder failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "seeded") {
		t.Fatalf("seeder did not report success:\n%s", out)
	}

	return sink.collected()
}

// TestSeedVolumeExceedsQueue keeps the delivery test honest. At a volume below
// the queue depth, an unchunked flush delivers everything anyway and the
// delivery test passes on broken code — which is exactly what happened the
// first time this file was written.
func TestSeedVolumeExceedsQueue(t *testing.T) {
	spans := seedCount * wantSpansPerTrace
	if spans <= maxQueueSize {
		t.Fatalf("test seeds %d spans against a %d-span queue; nothing would be dropped even without chunked flushing, so the delivery test proves nothing. Raise seedCount.",
			spans, maxQueueSize)
	}
}

func TestSeeder_DeliversEveryTrace(t *testing.T) {
	spans := runSeeder(t)

	var roots []receivedSpan
	for _, s := range spans {
		if s.parentID == "" {
			roots = append(roots, s)
		}
	}

	// The load-bearing assertion. seedCount*5 spans is well past the default
	// 2048-span queue, so an unchunked flush fails here rather than in
	// production.
	if len(roots) != seedCount {
		t.Errorf("received %d root spans, want %d — spans are being dropped between the SDK and the wire",
			len(roots), seedCount)
	}
	if want := seedCount * wantSpansPerTrace; len(spans) != want {
		t.Errorf("received %d spans total, want %d", len(spans), want)
	}
}

// TestSeeder_EmitsExpectedAttributes guards against the attribute-naming drift
// that the slides and the book depend on. If a rename lands here, the deck's
// BubbleUp screenshots and Masterclass 4's SLI stop matching the data.
func TestSeeder_EmitsExpectedAttributes(t *testing.T) {
	spans := runSeeder(t)

	seen := map[string]bool{}
	for _, s := range spans {
		if s.parentID != "" {
			continue
		}
		for k := range s.attrs {
			seen[k] = true
		}
	}

	for _, key := range []string{
		"http.request.method",
		"http.response.status_code",
		"http.route",
		"url.path",
		"user.id",
		"user.type",
		"service.version",
		"service.environment",
		"error",
		"error.type",
	} {
		if !seen[key] {
			t.Errorf("no root span carried %q; the deck and the saved queries reference it", key)
		}
	}

	// These are the names the book replaced. Their absence is the assertion.
	for _, stale := range []string{"http.method", "http.status_code", "customer.tier", "deployment.environment"} {
		if seen[stale] {
			t.Errorf("root spans carry %q, which the book's Chapter 6 tables do not use", stale)
		}
	}
}

// TestSeeder_RegressionIsVisibleOnTheWire checks that the demo's whole point
// survives serialization: the affected cohort really is slower once the data
// has been through the SDK and the exporter, not just in the generator.
func TestSeeder_RegressionIsVisibleOnTheWire(t *testing.T) {
	spans := runSeeder(t)

	var canary, baseline []uint64
	versions := map[string]int{}

	for _, s := range spans {
		if s.parentID != "" {
			continue
		}
		versions[s.attrs["service.version"]]++

		if s.attrs["http.route"] != "/api/billing" || s.attrs["user.type"] != "enterprise" {
			continue
		}
		switch s.attrs["service.version"] {
		case "1.5.0":
			canary = append(canary, s.durationNanos())
		case "1.4.2":
			baseline = append(baseline, s.durationNanos())
		}
	}

	if len(versions) != 2 {
		t.Fatalf("expected exactly two service.version values on the wire, got %v", versions)
	}
	if len(canary) == 0 {
		t.Fatal("no enterprise billing requests on the canary build; there is nothing for BubbleUp to find")
	}
	if len(baseline) == 0 {
		t.Fatal("no enterprise billing requests on the baseline build; there is no comparison to draw")
	}

	slices.Sort(canary)
	slices.Sort(baseline)
	canaryP50 := canary[len(canary)/2]
	baselineP50 := baseline[len(baseline)/2]

	ratio := float64(canaryP50) / float64(baselineP50)
	if ratio < 1.2 {
		t.Errorf("canary P50 is only %.2fx baseline for the affected cohort (%v vs %v); too subtle to demo",
			ratio, time.Duration(canaryP50), time.Duration(baselineP50))
	}
}
