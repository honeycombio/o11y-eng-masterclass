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
// inside it, for the same reason as session 4's: the BatchSpanProcessor
// silently drops spans that overflow its queue, and only a real run against a
// real OTLP receiver can catch that.
//
// seedCount must produce more than maxQueueSize spans (two per request) or
// this test cannot observe a dropped-span regression at all.
const seedCount = 4500

type receivedSpan struct {
	traceID  string
	name     string
	parentID string
	attrs    map[string]string
	status   int32
}

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
				var status int32
				if span.Status != nil {
					status = int32(span.Status.Code)
				}
				c.spans = append(c.spans, receivedSpan{
					traceID:  fmt.Sprintf("%x", span.TraceId),
					name:     span.Name,
					parentID: fmt.Sprintf("%x", span.ParentSpanId),
					attrs:    attrs,
					status:   status,
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

// runSeeder starts an in-process OTLP receiver, runs the seeder against it,
// and returns everything the receiver saw. SEED_SETTLE_WAIT is forced near
// zero: this receiver never samples, so there is nothing for the seeder's
// production wait (decision_delay + adjustment_interval, against the real
// Collector) to accomplish here.
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
		"SEED_SETTLE_WAIT=10ms",
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

// TestSeedVolumeExceedsQueue keeps the delivery test honest, same rationale
// as session 4's test of the same name: below the queue depth, an unchunked
// flush would deliver everything anyway and this test would pass on broken
// code.
func TestSeedVolumeExceedsQueue(t *testing.T) {
	spans := seedCount * 2 // two spans per request
	if spans <= maxQueueSize {
		t.Fatalf("test seeds %d requests (%d spans) against a %d-span queue; nothing would be dropped even without chunked flushing, so the delivery test proves nothing. Raise seedCount.",
			seedCount, spans, maxQueueSize)
	}
}

func TestSeeder_DeliversEveryRequest(t *testing.T) {
	spans := runSeeder(t)

	want := seedCount * 2
	if len(spans) != want {
		t.Errorf("received %d spans, want %d — spans are being dropped between the SDK and the wire", len(spans), want)
	}
}

// TestSeeder_EveryTraceHasTwoSpansWithAMatchingParent guards the shape the
// Collector's adaptive_tail_sampling processor depends on: it decides per
// trace, which only means something if every trace really does have a root
// and a child that share a trace, not two unrelated single-span traces.
func TestSeeder_EveryTraceHasTwoSpansWithAMatchingParent(t *testing.T) {
	spans := runSeeder(t)

	roots, children := 0, 0
	for _, s := range spans {
		if s.name == "db.query" {
			children++
			if s.parentID == "" {
				t.Errorf("db.query span has no parent — it should be a child of the root HTTP span")
			}
			continue
		}
		roots++
		if s.parentID != "" {
			t.Errorf("span %q has a parent %q; expected a root span", s.name, s.parentID)
		}
	}
	if roots != seedCount || children != seedCount {
		t.Fatalf("got %d root spans and %d db.query spans, want %d of each", roots, children, seedCount)
	}
}

// TestSeeder_EmitsExpectedAttributes guards against attribute-naming drift
// that the Collector's rules and the honeycomb-setup queries depend on.
func TestSeeder_EmitsExpectedAttributes(t *testing.T) {
	spans := runSeeder(t)

	seen := map[string]bool{}
	for _, s := range spans {
		for k := range s.attrs {
			seen[k] = true
		}
	}

	for _, key := range []string{
		"http.request.method",
		"http.route",
		"service.environment",
		"db.system",
	} {
		if !seen[key] {
			t.Errorf("no span carried %q", key)
		}
	}
}

// TestSeeder_ErrorRateIsVisibleOnTheWire checks the demo's other load-bearing
// fact survives serialization: a meaningful share of traces really do carry
// STATUS_CODE_ERROR (2), at a rate close to the configured 0.15, once the
// data has been through the SDK and the exporter, not just in the generator
// (TestGenerate_ErrorRateMatchesConfig already covers the generator itself).
func TestSeeder_ErrorRateIsVisibleOnTheWire(t *testing.T) {
	spans := runSeeder(t)

	const statusCodeError = 2
	traces := make(map[string][]receivedSpan, seedCount)
	for _, s := range spans {
		traces[s.traceID] = append(traces[s.traceID], s)
	}

	var errored int
	for traceID, ss := range traces {
		if len(ss) != 2 {
			t.Fatalf("trace %s has %d spans, want 2 (root + db.query)", traceID, len(ss))
		}
		e0, e1 := ss[0].status == statusCodeError, ss[1].status == statusCodeError
		if e0 != e1 {
			t.Fatalf("trace %s: root and child spans disagree on error status (%t vs %t) — the seeder's contract is both spans of an errored request carry STATUS_CODE_ERROR, which is what the Collector's keep-errors rule depends on", traceID, e0, e1)
		}
		if e0 {
			errored++
		}
	}

	rate := float64(errored) / float64(len(traces))
	if rate < 0.10 || rate > 0.20 {
		t.Errorf("error-status trace share on the wire is %.2f, want close to the configured 0.15", rate)
	}
}
