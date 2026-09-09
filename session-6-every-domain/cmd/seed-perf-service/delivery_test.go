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

// This is an end-to-end test of the shipped command rather than of a
// function inside it, for the same reason as every other seeder in this
// repo: the BatchSpanProcessor silently drops spans that overflow its
// queue, and only a real run against a real OTLP receiver can catch that.
//
// seedCount must exceed maxQueueSize, or this test cannot observe a
// dropped-span regression at all.
const seedCount = 9000

type receivedSpan struct {
	name  string
	attrs map[string]string
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
				c.spans = append(c.spans, receivedSpan{name: span.Name, attrs: attrs})
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

func TestSeedVolumeExceedsQueue(t *testing.T) {
	if seedCount <= maxQueueSize {
		t.Fatalf("test seeds %d requests (one span each) against a %d-span queue; nothing would be dropped even without chunked flushing, so the delivery test proves nothing. Raise seedCount.",
			seedCount, maxQueueSize)
	}
}

func TestSeeder_DeliversEveryRequest(t *testing.T) {
	spans := runSeeder(t)

	if len(spans) != seedCount {
		t.Errorf("received %d spans, want %d — spans are being dropped between the SDK and the wire", len(spans), seedCount)
	}
}

// TestSeeder_EmitsExpectedAttributes guards against attribute-naming drift
// that a Honeycomb query against this dataset would depend on.
func TestSeeder_EmitsExpectedAttributes(t *testing.T) {
	spans := runSeeder(t)

	seen := map[string]bool{}
	for _, s := range spans {
		for k := range s.attrs {
			seen[k] = true
		}
	}

	for _, key := range []string{"db.query.text", "user.type", "host.arch"} {
		if !seen[key] {
			t.Errorf("no span carried %q", key)
		}
	}
}

// TestSeeder_BothArchsAreVisibleOnTheWire checks the migration's before/after
// split survives serialization: both amd64 and arm64 spans actually exist.
func TestSeeder_BothArchsAreVisibleOnTheWire(t *testing.T) {
	spans := runSeeder(t)

	archCounts := map[string]int{}
	for _, s := range spans {
		archCounts[s.attrs["host.arch"]]++
	}

	if archCounts["amd64"] == 0 || archCounts["arm64"] == 0 {
		t.Fatalf("got %d amd64 and %d arm64 spans on the wire, want both non-zero", archCounts["amd64"], archCounts["arm64"])
	}
}

// TestSeeder_AllFiveQueryTemplatesAreVisibleOnTheWire checks Chapter 20's
// first workflow step has something to find: five distinct db.query.text
// values, not fewer.
func TestSeeder_AllFiveQueryTemplatesAreVisibleOnTheWire(t *testing.T) {
	spans := runSeeder(t)

	templates := map[string]bool{}
	for _, s := range spans {
		templates[s.attrs["db.query.text"]] = true
	}

	if len(templates) != 5 {
		t.Errorf("got %d distinct db.query.text values on the wire, want 5", len(templates))
	}
}
