// seed-sli-service backfills the Masterclass 4 dataset: a service that is
// mostly healthy, with an ongoing incident affecting enterprise customers on
// an older build. See internal/scenario for the scenario and why its
// proportions are what they are.
//
// Run this before the session, not during it. Because the incident is
// deliberately still "ongoing" relative to the seeded data's Now, re-run this
// shortly before you go live so the trailing-1-hour burn query and trigger
// have something current to find — a seed from yesterday reads as history,
// not an active incident.
//
// This dataset backs the free-tier workshop path: a honeycombio_derived_column
// implementing the SLI plus an AVG() query and a honeycombio_trigger, rather
// than a native SLO/burn-alert object (Honeycomb Free does not include SLOs).
// See ../../honeycomb-setup and ../../README.md.
package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strconv"
	"time"

	"github.com/honeycombio/o11y-eng-masterclass/session-4-slis-slos/internal/scenario"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

const serviceName = "sli-demo-service"

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
			cfg.RequestCount = n
		}
	}
	if v := os.Getenv("SEED_WINDOW"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Window = d
		}
	}
	if v := os.Getenv("SEED_INCIDENT_AGO"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.IncidentAgo = d
		}
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	requests := scenario.Generate(cfg, rng)

	// Emit in chunks, flushing after each. A backfill generates spans far
	// faster than the exporter ships them, and the BatchSpanProcessor silently
	// drops anything that overflows its queue — so without this, seeding a
	// large batch lands only a fraction of it in Honeycomb and reports
	// success. Chunking bounds the queue depth instead of trusting it.
	const chunk = 1000

	var incidentPop, incidentErrors int
	for i, req := range requests {
		emit(ctx, req)
		if req.Incident {
			incidentPop++
			if req.Errored {
				incidentErrors++
			}
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

	incidentStart := cfg.IncidentStart()
	fmt.Printf(`
seeded %d requests into dataset %q over the last %s
  incident window   started %s, still ongoing at seed time
  affected build     %s   (healthy build: %s)
  incident population %d requests (%d errored, %.0f%% observed rate)

The incident is anchored relative to now, so the trailing-1-hour SLI query and
the trigger in ../../honeycomb-setup only find it if you seed shortly before
you go live — a seed from yesterday reads as history, not a current burn.

Next: apply the Honeycomb Terraform (derived column, SLI query, trigger,
marker) so the workshop path has something to query and something to fire.

  cd ../../honeycomb-setup/terraform
  terraform apply

`,
		len(requests), serviceName, cfg.Window,
		incidentStart.Format(time.RFC3339),
		cfg.BuggyVersion, cfg.HealthyVersion,
		incidentPop, incidentErrors, 100*float64(incidentErrors)/float64(incidentPop),
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

// emit turns one generated request into a single root span at an explicit
// historical timestamp. Every request here is a standalone event rather than
// a multi-step trace — this dataset is about the SLI ratio over the whole
// population, not about drilling into one slow trace's waterfall, so there is
// nothing a child span would add.
func emit(ctx context.Context, req scenario.Request) {
	method := semconv.HTTPRequestMethodPost
	if req.Method == "GET" {
		method = semconv.HTTPRequestMethodGet
	}

	route := string(req.Route)
	_, span := tracer.Start(ctx, fmt.Sprintf("%s %s", req.Method, route),
		trace.WithTimestamp(req.Start))

	attrs := []attribute.KeyValue{
		method,
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
		span.SetStatus(codes.Error, req.ErrorType)
	}
	span.SetAttributes(attrs...)

	span.End(trace.WithTimestamp(req.Start.Add(req.Duration)))
}
