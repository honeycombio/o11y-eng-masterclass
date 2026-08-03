// otel-quickstart is the Masterclass 1 live demo, delivered in three beats:
//
//  1. Auto-instrumentation alone. otelhttp gives you POST /api/checkout with
//     about a dozen attributes and no code of your own.
//  2. One more attribute. order.id is a fact about the request, so it widens
//     the event that already exists rather than getting its own span.
//  3. One more span. Only once you want a duration measured separately from
//     the request's does a second event earn its place.
//
// An attribute describes; a span measures. Beats 2 and 3 ship commented out so
// `git checkout` restores the starting state and each beat is one uncomment.
//
// Reused as the `/api/checkout` service in later sessions (e.g. Masterclass 4's
// SLIs).
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

const serviceName = "otel-quickstart"

// tracer is only needed by beat 3. It sits here uncommented so that beat is a
// single uncomment in processOrder; an unused package-level var is legal Go.
var tracer = otel.Tracer(serviceName)

func main() {
	ctx := context.Background()

	shutdown, err := setupTracing(ctx)
	if err != nil {
		log.Fatalf("setting up tracing: %v", err)
	}
	defer shutdown(ctx)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealthz)
	mux.HandleFunc("POST /api/checkout", handleCheckout)

	handler := otelhttp.NewHandler(mux, serviceName)

	const addr = ":8080"
	log.Printf("otel-quickstart listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, handler))
}

func setupTracing(ctx context.Context) (func(context.Context) error, error) {
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		endpoint = "localhost:4317"
	}

	// WithInsecure because this always talks to the local Collector over
	// plaintext, never straight to Honeycomb; the Collector holds the API key.
	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("creating OTLP exporter: %w", err)
	}

	res, err := resource.Merge(resource.Default(),
		resource.NewSchemaless(semconv.ServiceName(serviceName)),
	)
	if err != nil {
		return nil, fmt.Errorf("building resource: %w", err)
	}

	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(provider)
	setupPropagation()

	return provider.Shutdown, nil
}

// setupPropagation registers the W3C trace context and baggage propagators.
//
// Without this, otel.GetTextMapPropagator() is a no-op: an inbound traceparent
// header is ignored and this service starts a brand-new trace instead of joining
// the caller's. That turns one distributed trace into two disconnected ones, and
// it fails silently — the spans still arrive and still look fine on their own.
//
// TraceContext is the W3C standard header pair. Baggage is what carries
// key-value context across a service boundary, which is the mechanism behind
// Chapter 7's async-boundary trap.
func setupPropagation() {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
}

// enrich adds attributes to the span otelhttp already created for this request,
// rather than starting a new one. This is the move Chapter 6 is about: widen the
// event you already have.
func enrich(ctx context.Context, kv ...attribute.KeyValue) {
	trace.SpanFromContext(ctx).SetAttributes(kv...)
}

// setRoute records http.route. otelhttp hands you about a dozen attributes for
// free, but not the matched route template — a concrete case of Chapter 6's
// "always review which attributes you get with automatic instrumentation, and
// don't settle for only what your instrumentation library gives you by
// default." http.route is also what Masterclass 4's SLI filters on, so it has
// to be here.
func setRoute(ctx context.Context, route string) {
	enrich(ctx, semconv.HTTPRoute(route))
}

func handleHealthz(w http.ResponseWriter, r *http.Request) {
	setRoute(r.Context(), "/healthz")
	w.WriteHeader(http.StatusOK)
}

func handleCheckout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	setRoute(ctx, "/api/checkout")
	orderID := newOrderID()

	// --- BEAT 2: one more attribute. Uncomment live. ---
	// order.id is a fact about this request, not a duration, so it widens the
	// span otelhttp already created. Wrapping it in a child span would add a
	// node to the trace without adding a measurement, and would split one
	// request's context across two narrower events instead of widening one.
	// enrich(ctx, attribute.String("order.id", orderID))
	// --- END BEAT 2 ---

	processOrder(ctx, orderID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"order_id": orderID})
}

// processOrder is beat 3. A span here is justified where the attribute alone was
// not: we want processing time as its own measurement, separate from total
// request duration. That is the question a span answers, and the reason to pay
// for a second event.
//
// order.id goes on this span as well, because attributes do not inherit down a
// trace. Each span is its own event, so a span you want to filter or group by
// order has to carry order.id itself — otherwise querying order.id finds the
// request and not the work done inside it.
func processOrder(ctx context.Context, orderID string) {
	// --- BEAT 3: one more span. Uncomment live. ---
	// ctx, span := tracer.Start(ctx, "process_order")
	// defer span.End()
	// enrich(ctx, attribute.String("order.id", orderID))
	// --- END BEAT 3 ---

	simulateOrderProcessing(ctx)
}

func simulateOrderProcessing(_ context.Context) {
	time.Sleep(time.Duration(20+rand.Intn(80)) * time.Millisecond)
}

func newOrderID() string {
	return fmt.Sprintf("ord_%d", rand.Int63())
}
