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

	"github.com/honeycombio/o11y-eng-masterclass/session-4-slis-slos/internal/scenario"
	collectortrace "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	"google.golang.org/grpc"
)

// This is an end-to-end test of the shipped command rather than of a function
// inside it, because the bug it exists to prevent lived precisely in the gap
// between the two.
//
// The BatchSpanProcessor silently drops spans that overflow its queue (2048 by
// default). An earlier seeder in this repo (session 2's) generated everything
// and flushed once at the end, so ~95% of it was dropped — and the command
// exited zero, printing a cheerful success message. Every unit test passed,
// because the generator was producing the data correctly. The data just never
// arrived. This test runs the real binary against a real OTLP receiver and
// counts what actually lands on the wire.

// seedCount must produce more spans than maxQueueSize, or this test cannot
// observe a dropped-span regression at all. TestSeedVolumeExceedsQueue enforces
// that rather than leaving it to a comment.
const seedCount = 9000

type receivedSpan struct {
	name     string
	parentID string
	attrs    map[string]string
	start    uint64
	end      uint64
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
// first time a seeder like this was written in this repo.
func TestSeedVolumeExceedsQueue(t *testing.T) {
	if seedCount <= maxQueueSize {
		t.Fatalf("test seeds %d requests (one span each) against a %d-span queue; nothing would be dropped even without chunked flushing, so the delivery test proves nothing. Raise seedCount.",
			seedCount, maxQueueSize)
	}
}

func TestSeeder_DeliversEveryRequest(t *testing.T) {
	spans := runSeeder(t)

	// The load-bearing assertion. seedCount spans (one per request, no child
	// spans in this dataset) is well past the default 2048-span queue, so an
	// unchunked flush fails here rather than in production.
	if len(spans) != seedCount {
		t.Errorf("received %d spans, want %d — spans are being dropped between the SDK and the wire", len(spans), seedCount)
	}
}

// TestSeeder_EmitsExpectedAttributes guards against the attribute-naming drift
// that the slides and the derived column depend on. If a rename lands here,
// the SLI's IF(LT($http.response.status_code, 500), ...) expression stops
// matching the data.
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
			t.Errorf("no span carried %q; the derived column and saved queries reference it", key)
		}
	}
}

// TestSeeder_HealthzAndIncidentAreVisibleOnTheWire checks that the demo's two
// load-bearing facts survive serialization: /healthz traffic actually exists
// (so the exclusion clause has something to exclude) and the incident cohort
// — buggy build, enterprise, inside the recent incident window — really does
// error at an elevated rate once the data has been through the SDK and the
// exporter, not just in the generator. The window matters here as much as in
// scenario_test.go: buggy-build-and-enterprise traffic from before the
// incident started is just ordinary background-rate traffic, so leaving out
// the recency filter would dilute the rate this test is checking.
func TestSeeder_HealthzAndIncidentAreVisibleOnTheWire(t *testing.T) {
	spans := runSeeder(t)

	cfg := scenario.DefaultConfig()
	// The seeder ran moments ago with cfg.Now == its own time.Now(), so an
	// incident window measured back from this test's time.Now() still
	// contains the same requests, with room to spare for the run time.
	incidentStart := uint64(time.Now().Add(-cfg.IncidentAgo).UnixNano())

	var healthz, incidentErrors, incidentTotal int
	for _, s := range spans {
		if s.attrs["http.route"] == "/healthz" {
			healthz++
			continue
		}
		if s.attrs["service.version"] != cfg.BuggyVersion || s.attrs["user.type"] != "enterprise" {
			continue
		}
		if s.start < incidentStart {
			continue
		}
		incidentTotal++
		if s.attrs["error"] == "true" {
			incidentErrors++
		}
	}

	if healthz == 0 {
		t.Error("no /healthz spans on the wire; the exclusion clause has nothing to demonstrate")
	}
	if incidentTotal == 0 {
		t.Fatalf("no service.version=%s + user.type=enterprise spans inside the incident window on the wire; nothing for BubbleUp or the burn query to find", cfg.BuggyVersion)
	}
	if rate := float64(incidentErrors) / float64(incidentTotal); rate < 0.3 {
		t.Errorf("incident cohort error rate is only %.2f on the wire; too subtle to demo", rate)
	}
}
