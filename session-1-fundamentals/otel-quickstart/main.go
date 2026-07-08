// otel-quickstart is the Masterclass 1 live demo: a checkout service that
// starts out auto-instrumented via otelhttp alone, then gets one hand-written
// span added for the part auto-instrumentation can't see — the business logic
// inside the handler. Reused as the `/api/checkout` service in later sessions
// (e.g. Masterclass 4's SLIs).
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
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

const serviceName = "otel-quickstart"

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

	return provider.Shutdown, nil
}

func handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func handleCheckout(w http.ResponseWriter, r *http.Request) {
	orderID := newOrderID()

	processOrder(r.Context(), orderID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"order_id": orderID})
}

// processOrder is where the live demo adds a custom span. otelhttp's
// auto-instrumentation already captures the HTTP request itself; everything
// inside this function is invisible to a trace until the block below exists.
func processOrder(ctx context.Context, orderID string) {
	// --- CUSTOM SPAN: added live, after showing auto-instrumentation alone ---
	ctx, span := tracer.Start(ctx, "process_order")
	span.SetAttributes(attribute.String("order.id", orderID))
	defer span.End()
	// --- END CUSTOM SPAN ---

	simulateOrderProcessing(ctx)
}

func simulateOrderProcessing(_ context.Context) {
	time.Sleep(time.Duration(20+rand.Intn(80)) * time.Millisecond)
}

func newOrderID() string {
	return fmt.Sprintf("ord_%d", rand.Int63())
}
