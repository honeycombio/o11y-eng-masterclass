// seed-sample-service backfills the "sample-service" Honeycomb dataset used
// in Masterclass 1's "one dataset, three ways" demo. Run this once, well
// before you go live — it emits historical-timestamped checkout traces
// (auth -> inventory_check -> payment -> confirmation) with a realistic
// error rate and a slow subset, so the count/rate view, the filtered search,
// and the trace waterfall all have something worth looking at.
package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strconv"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

const serviceName = "sample-service"

var tracer = otel.Tracer(serviceName)

func main() {
	ctx := context.Background()

	shutdown, err := setupTracing(ctx)
	if err != nil {
		log.Fatalf("setting up tracing: %v", err)
	}

	count := envInt("SEED_COUNT", 400)
	window := envDuration("SEED_WINDOW", 4*time.Hour)
	now := time.Now()
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	for i := 0; i < count; i++ {
		offset := time.Duration(rng.Int63n(int64(window)))
		simulateCheckout(ctx, rng, now.Add(-offset))
	}

	flushCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := shutdown(flushCtx); err != nil {
		log.Fatalf("flushing spans (is the Collector up? see ../collector): %v", err)
	}
	log.Printf("seeded %d checkout traces across the last %s into dataset %q", count, window, serviceName)
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

// simulateCheckout builds one checkout trace starting at start, with
// attributes drawn from the slide's four categories: identity (user.id,
// user.type), request/execution (http.request.method, http.route, url.path),
// outcome (http.response.status_code, error, error.type), and service/code
// context (service.version, service.environment).
//
// Attribute names follow the book's Chapter 6 tables rather than the deck's
// original wording: user.type (Table 6-15, not "customer.tier"),
// service.environment (Table 6-1, not the experimental
// "deployment.environment" that OTel has since renamed), and url.path
// (Table 6-8).
func simulateCheckout(ctx context.Context, rng *rand.Rand, start time.Time) {
	ctx, root := tracer.Start(ctx, "POST /api/checkout", trace.WithTimestamp(start))
	cursor := start

	rootAttrs := []attribute.KeyValue{
		semconv.HTTPRequestMethodPost,
		semconv.HTTPRoute("/api/checkout"),
		semconv.URLPath("/api/checkout"),
		attribute.String("user.id", fmt.Sprintf("user_%d", rng.Intn(5000))),
		attribute.String("user.type", weightedTier(rng)),
		attribute.String("service.version", "1.4.2"),
		// service.environment rather than deployment.environment: it's what the
		// book's Table 6-1 uses, and OTel has renamed its own experimental
		// deployment.environment to deployment.environment.name since.
		attribute.String("service.environment", "production"),
	}

	cursor = runStep(ctx, cursor, "auth", jitter(rng, 5*time.Millisecond, 15*time.Millisecond), nil, false)

	inventoryDuration := jitter(rng, 10*time.Millisecond, 30*time.Millisecond)
	if rng.Float64() < 0.12 { // cold cache: the slow subset the demo's waterfall/heatmap view relies on
		inventoryDuration = jitter(rng, 300*time.Millisecond, 800*time.Millisecond)
	}
	cursor = runStep(ctx, cursor, "inventory_check", inventoryDuration, []attribute.KeyValue{
		attribute.Int64("db.query.duration_ms", inventoryDuration.Milliseconds()),
	}, false)

	paymentFailed := rng.Float64() < 0.07
	paymentDuration := jitter(rng, 20*time.Millisecond, 60*time.Millisecond)
	paymentAttrs := []attribute.KeyValue{
		attribute.Int64("db.query.duration_ms", paymentDuration.Milliseconds()),
	}
	cursor = runStep(ctx, cursor, "payment", paymentDuration, paymentAttrs, paymentFailed)

	if paymentFailed {
		rootAttrs = append(rootAttrs,
			semconv.HTTPResponseStatusCode(402),
			attribute.Bool("error", true),
			attribute.String("error.type", "payment_declined"),
		)
		root.SetStatus(codes.Error, "payment_declined")
	} else {
		cursor = runStep(ctx, cursor, "confirmation", jitter(rng, 5*time.Millisecond, 10*time.Millisecond), nil, false)
		rootAttrs = append(rootAttrs, semconv.HTTPResponseStatusCode(201))
	}

	root.SetAttributes(rootAttrs...)
	root.End(trace.WithTimestamp(cursor))
}

// runStep starts and ends a child span at explicit historical timestamps,
// returning the timestamp the next step should start at.
func runStep(ctx context.Context, start time.Time, name string, duration time.Duration, attrs []attribute.KeyValue, failed bool) time.Time {
	end := start.Add(duration)

	_, span := tracer.Start(ctx, name, trace.WithTimestamp(start))
	span.SetAttributes(attrs...)
	if failed {
		span.SetStatus(codes.Error, name+" failed")
	}
	span.End(trace.WithTimestamp(end))

	return end
}

func weightedTier(rng *rand.Rand) string {
	switch r := rng.Float64(); {
	case r < 0.6:
		return "free"
	case r < 0.9:
		return "premium"
	default:
		return "enterprise"
	}
}

func jitter(rng *rand.Rand, min, max time.Duration) time.Duration {
	return min + time.Duration(rng.Int63n(int64(max-min)))
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func envDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}
