// seed-arbitrary-question backfills the Masterclass 3 dataset: a week of
// genuinely wide checkout events, used to run Chapter 28's Arbitrary Question
// Test live.
//
// Run this before the session. It writes to its own dataset rather than the
// sample-service one Masterclass 1 and 2 use, so rehearsing all three sessions
// in a row does not put a week of differently-shaped data inside their 4-hour
// windows.
//
// On completion it prints the question to ask and how many events actually
// answer it, so you know before going live that the result will not be empty.
package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strconv"
	"time"

	"github.com/honeycombio/o11y-eng-masterclass/session-3-business-case/internal/wideevent"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

const serviceName = "checkout-web"

// maxQueueSize is the BatchSpanProcessor queue depth. Named rather than inlined
// because delivery_test.go asserts against it: a test seeding fewer spans than
// this cannot detect a dropped-span regression at all.
const maxQueueSize = 8192

var tracer = otel.Tracer(serviceName)

func main() {
	ctx := context.Background()

	provider, err := setupTracing(ctx)
	if err != nil {
		log.Fatalf("setting up tracing: %v", err)
	}

	cfg := wideevent.DefaultConfig()
	cfg.Now = time.Now()
	if v := os.Getenv("SEED_COUNT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.EventCount = n
		}
	}
	if v := os.Getenv("SEED_WINDOW"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Window = d
		}
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	events := wideevent.Generate(cfg, rng)

	// Chunked flushing, for the same reason as the other seeders: the
	// BatchSpanProcessor silently drops whatever overflows its queue, so a
	// single trailing flush loses most of a backfill and still exits zero.
	const chunk = 500

	var matches int
	for i, e := range events {
		emit(ctx, e)
		if e.MatchesArbitraryQuestion() {
			matches++
		}
		if (i+1)%chunk == 0 {
			if err := flush(ctx, provider); err != nil {
				log.Fatalf("flushing spans after %d events: %v", i+1, err)
			}
		}
	}

	if err := flush(ctx, provider); err != nil {
		log.Fatalf("final flush: %v", err)
	}
	if err := provider.Shutdown(ctx); err != nil {
		log.Fatalf("shutting down tracer provider: %v", err)
	}

	fmt.Printf(`
seeded %d checkout events into dataset %q over the last %s
  attributes per event   %d
  matching the question  %d

The Arbitrary Question Test (Chapter 28). Ask:

  "Show me all failed checkout attempts from mobile users in California
   using version %s of the app during lunch hour over the past week."

In Honeycomb, that is a COUNT with these filters:

  error                  = true
  user_agent.device      = %s          <- "mobile" is device=phone in the convention
  geo.region.iso_code    = %s          <- California, ISO 3166-2
  user_agent.app_version = %s
  and a %d:00-%d:00 time-of-day filter

Expect roughly %d results. If it comes back empty, reseed before going live.

`,
		len(events), serviceName, cfg.Window,
		len(wideevent.Attributes(events[0])),
		matches,
		wideevent.AnswerVersion,
		wideevent.AnswerDevice,
		wideevent.AnswerRegion,
		wideevent.AnswerVersion,
		wideevent.LunchHourStart, wideevent.LunchHourEnd,
		matches,
	)
}

func flush(ctx context.Context, provider *sdktrace.TracerProvider) error {
	flushCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := provider.ForceFlush(flushCtx); err != nil {
		return fmt.Errorf("%w (is the Collector up? see ../../../session-1-fundamentals/collector)", err)
	}
	return nil
}

func setupTracing(ctx context.Context) (*sdktrace.TracerProvider, error) {
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
		sdktrace.WithBatcher(exporter,
			sdktrace.WithMaxQueueSize(maxQueueSize),
			sdktrace.WithMaxExportBatchSize(1024),
		),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(provider)

	return provider, nil
}

// emit writes one wide event as a single span. There are no child spans here on
// purpose: this dataset exists to demonstrate querying across many attributes,
// which is the canonical-log shape rather than the trace shape.
func emit(ctx context.Context, e wideevent.Event) {
	_, span := tracer.Start(ctx, "POST "+e.Route, trace.WithTimestamp(e.Start))
	span.SetAttributes(wideevent.Attributes(e)...)
	if e.Errored {
		span.SetStatus(codes.Error, e.ErrorType)
	}
	span.End(trace.WithTimestamp(e.Start.Add(e.Duration)))
}
