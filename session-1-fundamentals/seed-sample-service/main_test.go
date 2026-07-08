package main

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

// TestSimulateCheckout_TraceInvariants runs many seeded checkouts through an
// in-memory exporter and checks the invariants the live demo depends on:
// every trace has exactly one root, every other span is its child, and the
// root's error state agrees with whether payment failed. Fixed seed makes
// this deterministic while still exercising both the success and failure
// branches, plus the slow-inventory branch.
func TestSimulateCheckout_TraceInvariants(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	t.Cleanup(func() { otel.SetTracerProvider(prev) })

	const iterations = 500
	rng := rand.New(rand.NewSource(42))
	for i := 0; i < iterations; i++ {
		simulateCheckout(context.Background(), rng, time.Now())
	}
	if err := provider.ForceFlush(context.Background()); err != nil {
		t.Fatalf("force flush: %v", err)
	}

	spans := exporter.GetSpans()

	roots := make(map[trace.SpanID]tracetest.SpanStub)
	children := make(map[trace.SpanID][]tracetest.SpanStub)
	for _, s := range spans {
		if !s.Parent.IsValid() {
			roots[s.SpanContext.SpanID()] = s
			continue
		}
		children[s.Parent.SpanID()] = append(children[s.Parent.SpanID()], s)
	}

	if len(roots) != iterations {
		t.Fatalf("got %d root spans, want %d", len(roots), iterations)
	}

	var sawSuccess, sawFailure, sawSlowInventory bool
	for spanID, root := range roots {
		statusCode, hasErrorAttr := findIntAttr(root.Attributes, "http.response.status_code")
		if !hasErrorAttr {
			t.Fatalf("root span %s missing http.response.status_code", spanID)
		}

		kids := children[spanID]
		wantErrorTrace := statusCode == 402
		if wantErrorTrace {
			sawFailure = true
			if root.Status.Code != codes.Error {
				t.Errorf("trace with http.response.status_code=402 has span status %v, want Error", root.Status.Code)
			}
			if len(kids) != 3 {
				t.Errorf("failed checkout has %d child spans (auth, inventory_check, payment), want 3", len(kids))
			}
		} else {
			sawSuccess = true
			if root.Status.Code == codes.Error {
				t.Errorf("trace with http.response.status_code=%d has span status Error, want Unset", statusCode)
			}
			if len(kids) != 4 {
				t.Errorf("successful checkout has %d child spans (auth, inventory_check, payment, confirmation), want 4", len(kids))
			}
		}

		for _, kid := range kids {
			if kid.EndTime.Before(kid.StartTime) {
				t.Errorf("span %s ends before it starts", kid.Name)
			}
			if ms, ok := findIntAttr(kid.Attributes, "db.query.duration_ms"); ok && kid.Name == "inventory_check" && ms > 200 {
				sawSlowInventory = true
			}
		}
	}

	if !sawSuccess {
		t.Error("500 iterations at a 7% failure rate produced zero successful checkouts")
	}
	if !sawFailure {
		t.Error("500 iterations at a 7% failure rate produced zero failed checkouts")
	}
	if !sawSlowInventory {
		t.Error("500 iterations at a 12% slow rate produced zero slow inventory_check spans")
	}
}

func findIntAttr(attrs []attribute.KeyValue, key string) (int64, bool) {
	for _, a := range attrs {
		if string(a.Key) == key {
			return a.Value.AsInt64(), true
		}
	}
	return 0, false
}
