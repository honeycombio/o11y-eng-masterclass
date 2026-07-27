// seed-canary-regression backfills the Masterclass 2 dataset: a canary deploy
// that is ~40% slower, but only for enterprise users on the billing endpoint.
// See internal/scenario for the scenario and why its proportions are what they
// are.
//
// Run this before the session, not during it. It writes into the same
// sample-service dataset Masterclass 1 uses, so the deploy marker and the
// before/after queries land on data attendees already recognise.
//
// On completion it prints the exact deploy timestamp, which the Honeycomb
// deploy marker must match — see ../../honeycomb-setup.
package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strconv"
	"time"

	"github.com/honeycombio/o11y-eng-masterclass/session-2-feedback-loops/internal/scenario"
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

// maxQueueSize is the BatchSpanProcessor queue depth. Named rather than inlined
// because the delivery test needs it: a test that seeds fewer spans than this
// cannot detect a dropped-span regression, so it asserts against this value
// rather than a hardcoded guess that would silently rot if this changed.
const maxQueueSize = 8192

var tracer = otel.Tracer(serviceName)

func main() {
	ctx := context.Background()

	provider, err := setupTracing(ctx)
	if err != nil {
		log.Fatalf("setting up tracing: %v", err)
	}

	cfg := scenario.DefaultConfig()
	cfg.Now = time.Now()
	if v := os.Getenv("SEED_COUNT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.TraceCount = n
		}
	}
	if v := os.Getenv("SEED_WINDOW"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Window = d
		}
	}
	if v := os.Getenv("SEED_DEPLOY_AGO"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.DeployAgo = d
		}
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	requests := scenario.Generate(cfg, rng)

	// Emit in chunks, flushing after each. A backfill generates spans far
	// faster than the exporter ships them, and the BatchSpanProcessor silently
	// drops anything that overflows its queue — so without this, seeding 10,000
	// requests lands roughly 500 of them in Honeycomb and reports success.
	// Chunking bounds the queue depth instead of trusting it.
	const chunk = 400

	var regressed int
	for i, req := range requests {
		emit(ctx, req)
		if req.Regressed {
			regressed++
		}
		if (i+1)%chunk == 0 {
			if err := flush(ctx, provider); err != nil {
				log.Fatalf("flushing spans after %d requests: %v", i+1, err)
			}
		}
	}

	if err := flush(ctx, provider); err != nil {
		log.Fatalf("final flush: %v", err)
	}
	if err := provider.Shutdown(ctx); err != nil {
		log.Fatalf("shutting down tracer provider: %v", err)
	}

	deploy := cfg.DeployTime()
	fmt.Printf(`
seeded %d requests into dataset %q over the last %s
  canary build     %s  (%d requests regressed, %.2f%% of traffic)
  baseline build   %s
  deploy time      %s

Next: create the deploy marker at that exact time, or the before/after
boundary will not line up with the data.

  cd ../../honeycomb-setup/scripts
  HONEYCOMB_API_KEY=<config-permission key> ./create-deploy-marker.sh %d

`,
		len(requests), serviceName, cfg.Window,
		cfg.CanaryVersion, regressed, 100*float64(regressed)/float64(len(requests)),
		cfg.BaselineVersion,
		deploy.Format(time.RFC3339),
		deploy.Unix(),
	)
}

// flush blocks until the exporter has shipped everything queued so far, so the
// caller can safely generate the next chunk.
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
		// A queue well above the 2048 default, so a chunk never comes close to
		// overflowing it even if an export is briefly slow.
		sdktrace.WithBatcher(exporter,
			sdktrace.WithMaxQueueSize(maxQueueSize),
			sdktrace.WithMaxExportBatchSize(1024),
		),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(provider)

	return provider, nil
}

// emit turns one generated request into a root span plus its child spans, at
// explicit historical timestamps. Attribute names match Masterclass 1's
// dataset: the book's Chapter 6 tables where the book has an opinion,
// current stable OTel semantic conventions otherwise.
func emit(ctx context.Context, req scenario.Request) {
	route := string(req.Route)

	ctx, root := tracer.Start(ctx,
		fmt.Sprintf("POST %s", route),
		trace.WithTimestamp(req.Start),
	)

	attrs := []attribute.KeyValue{
		semconv.HTTPRequestMethodPost,
		semconv.HTTPRoute(route),
		semconv.URLPath(route),
		semconv.HTTPResponseStatusCode(req.StatusCode()),
		attribute.String("user.id", fmt.Sprintf("user_%d", req.UserID)),
		attribute.String("user.type", req.UserType),
		attribute.String("service.version", req.Version),
		attribute.String("service.environment", "production"),
	}
	if req.Errored {
		attrs = append(attrs,
			attribute.Bool("error", true),
			attribute.String("error.type", req.ErrorType),
		)
		root.SetStatus(codes.Error, req.ErrorType)
	}
	root.SetAttributes(attrs...)

	cursor := req.Start
	for _, step := range req.Steps {
		end := cursor.Add(step.Duration)
		// No duration attribute here on purpose. A span's duration is already
		// computed from its start and end timestamps, and Honeycomb surfaces it
		// as duration_ms — so emitting an attribute by that name shadows the
		// real field in the one demo that is entirely about reading durations.
		// Masterclass 1 writes into this same dataset and uses
		// db.query.duration_ms for a genuinely distinct measurement; a second
		// spelling of "how long did this take" is exactly the ontology drift
		// Chapter 7 warns about.
		_, span := tracer.Start(ctx, step.Name, trace.WithTimestamp(cursor))
		span.End(trace.WithTimestamp(end))
		cursor = end
	}

	root.End(trace.WithTimestamp(cursor))
}
