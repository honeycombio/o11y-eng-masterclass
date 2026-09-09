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
// inside it, for the same reason as every other seeder in this repo: the
// BatchSpanProcessor silently drops spans that overflow its queue, and only
// a real run against a real OTLP receiver can catch that.
//
// seedCount (runs) * spansPerRun must exceed maxQueueSize, or this test
// cannot observe a dropped-span regression at all.
const seedCount = 350

// spansPerRun matches internal/cicdscenario's DefaultConfig TestCount (20):
// 1 root + 4 tasks + 20 test cases.
const spansPerRun = 1 + 4 + 20

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

// TestSeedVolumeExceedsQueue keeps the delivery test honest, same rationale
// as every other seeder in this repo.
func TestSeedVolumeExceedsQueue(t *testing.T) {
	spans := seedCount * spansPerRun
	if spans <= maxQueueSize {
		t.Fatalf("test seeds %d runs (%d spans) against a %d-span queue; nothing would be dropped even without chunked flushing, so the delivery test proves nothing. Raise seedCount.",
			seedCount, spans, maxQueueSize)
	}
}

func TestSeeder_DeliversEveryRun(t *testing.T) {
	spans := runSeeder(t)

	want := seedCount * spansPerRun
	if len(spans) != want {
		t.Errorf("received %d spans, want %d — spans are being dropped between the SDK and the wire", len(spans), want)
	}
}

// TestSeeder_EveryRunIsOneTraceWithFiveTaskLevelSpans checks the shape a
// flakiness/duration query depends on: one trace per run, a root plus four
// task children, with the test task itself parenting its test-case spans.
func TestSeeder_EveryRunIsOneTraceWithFiveTaskLevelSpans(t *testing.T) {
	spans := runSeeder(t)

	traces := make(map[string][]receivedSpan, seedCount)
	for _, s := range spans {
		traces[s.traceID] = append(traces[s.traceID], s)
	}

	if len(traces) != seedCount {
		t.Fatalf("got %d distinct traces, want %d — one per run", len(traces), seedCount)
	}

	for traceID, ss := range traces {
		if len(ss) != spansPerRun {
			t.Fatalf("trace %s has %d spans, want %d", traceID, len(ss), spansPerRun)
		}
		var roots, tasks, tests int
		for _, s := range ss {
			switch {
			case s.parentID == "":
				roots++
			case s.attrs["cicd.pipeline.task.name"] != "":
				tasks++
			default:
				tests++
			}
		}
		if roots != 1 || tasks != 4 || tests != 20 {
			t.Fatalf("trace %s: %d roots, %d tasks, %d test cases; want 1, 4, 20", traceID, roots, tasks, tests)
		}
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

	for _, key := range []string{
		"cicd.pipeline.name",
		"cicd.pipeline.run.id",
		"vcs.change.id",
		"cicd.pipeline.task.name",
		"test.name",
	} {
		if !seen[key] {
			t.Errorf("no span carried %q", key)
		}
	}
}

// TestSeeder_BuildTaskSpansAreOnTheWire checks the build task survives
// serialization at all — the scenario package's own tests
// (TestGenerate_RegressionIsAStepChange) already cover the duration-shape
// claim precisely against the generator's output; this test's narrower job
// is confirming the wire format (this test harness's attrs map doesn't carry
// span start/end times to check duration directly).
func TestSeeder_BuildTaskSpansAreOnTheWire(t *testing.T) {
	spans := runSeeder(t)

	var buildSpans int
	for _, s := range spans {
		if s.attrs["cicd.pipeline.task.name"] == "build" {
			buildSpans++
		}
	}
	if buildSpans != seedCount {
		t.Fatalf("got %d build-task spans on the wire, want %d (one per run)", buildSpans, seedCount)
	}
}

// TestSeeder_FlakyTestFailureIsVisibleOnTheWire checks the other load-bearing
// fact survives serialization: the flaky test case really does fail some of
// the time, with a real STATUS_CODE_ERROR (2) on the wire.
func TestSeeder_FlakyTestFailureIsVisibleOnTheWire(t *testing.T) {
	spans := runSeeder(t)

	const statusCodeError = 2
	var flakyTotal, flakyFailed int
	for _, s := range spans {
		if s.attrs["test.name"] != "TestPaymentRetryIsIdempotent" {
			continue
		}
		flakyTotal++
		if s.status == statusCodeError {
			flakyFailed++
		}
	}

	if flakyTotal != seedCount {
		t.Fatalf("got %d flaky-test spans, want %d (one per run)", flakyTotal, seedCount)
	}
	if flakyFailed == 0 {
		t.Error("flaky test never failed on the wire across the whole seed; nothing for a flakiness query to find")
	}
	if flakyFailed == flakyTotal {
		t.Error("flaky test failed on every run; that's a broken test, not a flaky one")
	}
}
