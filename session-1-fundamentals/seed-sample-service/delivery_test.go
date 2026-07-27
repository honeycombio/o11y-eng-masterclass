package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	collectortrace "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/grpc"
)

// The scenario tests above run through an in-memory exporter, which proves the
// traces are *built* correctly. This one proves they arrive.
//
// The distinction matters because it is exactly where a real bug hid: the
// BatchSpanProcessor silently drops spans past its queue limit (2048 by
// default), so a version of this seeder that flushed once at the end delivered
// a fraction of its data and still exited zero. This file's default seed count
// puts it comfortably over that limit, so a regression fails here.
//
// It is also the case that this seeder's README invites raising SEED_COUNT,
// which is precisely the scenario that used to lose data.

// Must produce more spans than maxQueueSize, or an unchunked flush would
// deliver everything anyway and this test would pass on broken code.
// TestDeliverySeedVolumeExceedsQueue enforces it.
const deliverySeedCount = 2500

// minSpansPerTrace is the root plus three children; a successful checkout adds a
// fourth. The lower bound is what matters for the queue comparison.
const minSpansPerTrace = 4

type deliveryCollector struct {
	collectortrace.UnimplementedTraceServiceServer
	mu    sync.Mutex
	roots int
	total int
}

func (c *deliveryCollector) Export(_ context.Context, req *collectortrace.ExportTraceServiceRequest) (*collectortrace.ExportTraceServiceResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, rs := range req.ResourceSpans {
		for _, ss := range rs.ScopeSpans {
			for _, span := range ss.Spans {
				c.total++
				if len(span.ParentSpanId) == 0 {
					c.roots++
				}
			}
		}
	}
	return &collectortrace.ExportTraceServiceResponse{}, nil
}

// TestDeliverySeedVolumeExceedsQueue keeps the test below it honest.
func TestDeliverySeedVolumeExceedsQueue(t *testing.T) {
	if spans := deliverySeedCount * minSpansPerTrace; spans <= maxQueueSize {
		t.Fatalf("test seeds at least %d spans against a %d-span queue; nothing would be dropped even without chunked flushing. Raise deliverySeedCount.",
			spans, maxQueueSize)
	}
}

func TestSeeder_DeliversEveryTraceOverOTLP(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	sink := &deliveryCollector{}
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
		fmt.Sprintf("SEED_COUNT=%d", deliverySeedCount),
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("seeder failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "seeded") {
		t.Fatalf("seeder did not report success:\n%s", out)
	}

	sink.mu.Lock()
	roots, total := sink.roots, sink.total
	sink.mu.Unlock()

	if roots != deliverySeedCount {
		t.Errorf("received %d root spans, want %d — spans are being dropped between the SDK and the wire",
			roots, deliverySeedCount)
	}
	// Four or five spans per trace depending on whether payment failed, so
	// assert a band rather than an exact count.
	if minSpans := deliverySeedCount * minSpansPerTrace; total < minSpans {
		t.Errorf("received %d spans total, want at least %d", total, minSpans)
	}
}
