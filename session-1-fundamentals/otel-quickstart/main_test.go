package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// exporter is shared by every test in this package. otel's global tracer
// delegate can be set exactly once per process, so a second TracerProvider is
// silently ignored and its spans keep landing in the first exporter. One
// provider, and each test takes the slice of spans its own request appended.
var exporter *tracetest.InMemoryExporter

func TestMain(m *testing.M) {
	exporter = tracetest.NewInMemoryExporter()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter)))
	setupPropagation()
	os.Exit(m.Run())
}

// newHandler mirrors what main() serves, so these tests exercise the real
// wiring rather than a copy of it that can drift.
func newHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealthz)
	mux.HandleFunc("POST /api/checkout", handleCheckout)
	return otelhttp.NewHandler(mux, serviceName)
}

// spansFor drives one request through the real handler and returns only the
// spans that request produced. Tests must not call t.Parallel: the before/after
// slice boundary assumes sequential execution.
func spansFor(t *testing.T, req *http.Request, wantStatus int) []tracetest.SpanStub {
	t.Helper()

	before := len(exporter.GetSpans())
	rec := httptest.NewRecorder()
	newHandler().ServeHTTP(rec, req)
	if rec.Code != wantStatus {
		t.Fatalf("%s %s returned %d, want %d", req.Method, req.URL.Path, rec.Code, wantStatus)
	}

	spans := exporter.GetSpans()[before:]
	if len(spans) == 0 {
		t.Fatal("no spans exported")
	}
	return spans
}

// TestCheckoutTraceShape pins the properties that must hold at every beat of the
// demo, including with beats 2 and 3 still commented out as committed.
//
// Span count deliberately is not asserted: beat 3 legitimately adds one. What
// must never change is where order.id lives. An attribute describing the request
// belongs on the request's own span, so if it ever migrates onto a child this
// fails — that is the distinction the demo teaches.
func TestCheckoutTraceShape(t *testing.T) {
	spans := spansFor(t, httptest.NewRequest("POST", "/api/checkout", nil), http.StatusCreated)

	var roots, children []tracetest.SpanStub
	for _, s := range spans {
		if s.Parent.IsValid() {
			children = append(children, s)
		} else {
			roots = append(roots, s)
		}
	}

	// One root, always. A request with no inbound traceparent starts its own
	// trace; if the request span ever gained a parent here, every trace in the
	// dataset would render with phantom missing spans above it.
	if len(roots) != 1 {
		t.Fatalf("got %d root spans, want 1", len(roots))
	}
	root := roots[0]

	// otelhttp renames the span from r.Pattern after routing. This guards
	// against a change that reverts it to the static operation name.
	if want := "POST /api/checkout"; root.Name != want {
		t.Errorf("root span name = %q, want %q", root.Name, want)
	}

	rootAttrs := map[string]string{}
	for _, kv := range root.Attributes {
		rootAttrs[string(kv.Key)] = kv.Value.String()
	}

	// Masterclass 4's SLI filters on http.route, and otelhttp does not set it
	// for a mux-level wrap — see setRoute.
	if got := rootAttrs["http.route"]; got != "/api/checkout" {
		t.Errorf("http.route on root = %q, want %q", got, "/api/checkout")
	}

	rootOrderID, rootHasOrderID := rootAttrs["order.id"]

	// Children are allowed (beat 3), but each must hang off the request span and
	// carry order.id itself. Attributes do not inherit down a trace, so a child
	// without it cannot be filtered or grouped by order — querying order.id
	// would find the request and not the work done inside it.
	for _, c := range children {
		if c.Parent.SpanID() != root.SpanContext.SpanID() {
			t.Errorf("span %q parented to %s, want the request span %s",
				c.Name, c.Parent.SpanID(), root.SpanContext.SpanID())
		}

		var childOrderID string
		var found bool
		for _, kv := range c.Attributes {
			if kv.Key == "order.id" {
				childOrderID, found = kv.Value.String(), true
			}
		}

		switch {
		case found && !rootHasOrderID:
			t.Errorf("span %q carries order.id but the request span does not; the attribute describes the request, so it belongs there first", c.Name)
		case rootHasOrderID && !found:
			t.Errorf("span %q is missing order.id; attributes do not inherit, so this span cannot be sliced by order", c.Name)
		case found && childOrderID != rootOrderID:
			t.Errorf("span %q has order.id %q, want %q to match the request span", c.Name, childOrderID, rootOrderID)
		}
	}

	switch {
	case rootHasOrderID && len(children) > 0:
		t.Logf("beat 3 state: order.id on the request span and on %d child span(s)", len(children))
	case rootHasOrderID:
		t.Logf("beat 2 state: order.id on the request span, no child spans")
	default:
		t.Logf("beat 1 state (as committed): auto-instrumentation only, %d child span(s)", len(children))
	}
}

// TestInboundTraceparentIsJoined covers a failure that is invisible in the
// resulting data: with no propagator registered, an inbound traceparent is
// dropped and this service silently starts a second, disconnected trace rather
// than continuing the caller's.
func TestInboundTraceparentIsJoined(t *testing.T) {
	const (
		upstreamTrace = "4bf92f3577b34da6a3ce929d0e0e4736"
		upstreamSpan  = "00f067aa0ba902b7"
	)

	req := httptest.NewRequest("POST", "/api/checkout", nil)
	req.Header.Set("traceparent", "00-"+upstreamTrace+"-"+upstreamSpan+"-01")

	var request *tracetest.SpanStub
	for _, s := range spansFor(t, req, http.StatusCreated) {
		if s.Name == "POST /api/checkout" {
			request = &s
			break
		}
	}
	if request == nil {
		t.Fatal(`no "POST /api/checkout" span found`)
	}

	if got := request.SpanContext.TraceID().String(); got != upstreamTrace {
		t.Errorf("trace ID = %s, want the caller's %s — the inbound traceparent was not extracted, so this is a new trace", got, upstreamTrace)
	}
	if got := request.Parent.SpanID().String(); got != upstreamSpan {
		t.Errorf("parent span ID = %s, want the caller's %s", got, upstreamSpan)
	}
	if !request.Parent.IsRemote() {
		t.Error("parent is not marked remote; it should have come from the traceparent header")
	}
}
