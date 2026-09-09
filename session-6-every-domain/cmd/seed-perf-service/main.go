// seed-perf-service backfills the Masterclass 6 performance-engineering
// dataset: five parameterized db.query.text templates (three fast, one
// bimodal, one long-tail), a user.type correlation, and a Graviton
// (amd64 -> arm64) migration marker halfway through the window with no
// latency regression across it. See internal/perfscenario for the scenario
// and why its proportions are what they are.
//
// Run this against the Collector from session-1-fundamentals/collector, same
// as every other session.
package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strconv"
	"time"

	"github.com/honeycombio/o11y-eng-masterclass/session-6-every-domain/internal/perfscenario"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.41.0"
	"go.opentelemetry.io/otel/trace"
)

const serviceName = "perf-service"

// maxQueueSize is the BatchSpanProcessor queue depth. Named rather than
// inlined because the delivery test needs it — same rationale as every
// other seeder in this repo.
const maxQueueSize = 8192

var tracer = otel.Tracer(serviceName)

func main() {
	ctx := context.Background()

	provider, err := setupTracing(ctx)
	if err != nil {
		log.Fatalf("setting up tracing: %v", err)
	}

	cfg := perfscenario.DefaultConfig()
	cfg.Now = time.Now()
	if v := os.Getenv("SEED_COUNT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.RequestCount = n
		}
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	requests := perfscenario.Generate(cfg, rng)

	const chunk = 1000

	var amd64Reqs, arm64Reqs []perfscenario.Request
	for i, req := range requests {
		emit(ctx, req)
		if req.Arch == "amd64" {
			amd64Reqs = append(amd64Reqs, req)
		} else {
			arm64Reqs = append(arm64Reqs, req)
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

	migrationStart := cfg.MigrationStart()
	fmt.Printf(`
seeded %d requests into service %q
  migration          completed %s (amd64 before, arm64 after)
  amd64 requests     %d   P50 %s   P95 %s
  arm64 requests     %d   P50 %s   P95 %s

Compare the P50/P95 pairs above: that's the "no latency regression" claim
this seeder is built to make honestly. See ../../honeycomb-setup for the
saved queries that reproduce this comparison in Honeycomb itself.
`,
		len(requests), serviceName,
		migrationStart.Format(time.RFC3339),
		len(amd64Reqs), perfscenario.Percentile(amd64Reqs, 50), perfscenario.Percentile(amd64Reqs, 95),
		len(arm64Reqs), perfscenario.Percentile(arm64Reqs, 50), perfscenario.Percentile(arm64Reqs, 95),
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

// emit turns one generated request into a single root span at an explicit
// historical timestamp — this dataset is about query duration analysis
// across a large population, not a trace waterfall, so there is nothing a
// child span would add. Same reasoning as session 4's single-span seeder.
func emit(ctx context.Context, req perfscenario.Request) {
	_, span := tracer.Start(ctx, "db.query", trace.WithTimestamp(req.Start))
	span.SetAttributes(
		attribute.String("db.query.text", req.QueryText),
		attribute.String("user.type", req.UserType),
		semconv.HostArchKey.String(req.Arch),
	)
	span.End(trace.WithTimestamp(req.Start.Add(req.Duration)))
}
