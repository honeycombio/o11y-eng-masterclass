// seed-cost-sampling-service generates the Masterclass 5 dataset: a mostly-
// healthy service with a skewed route distribution and a synthetic 15% error
// rate, emitted in real time (not backdated, unlike session 4's seeder) so
// session-1's shared Collector actually makes a live tail-sampling decision
// on each trace as it arrives. See internal/scenario for the scenario and
// why its proportions are what they are, and
// ../../session-1-fundamentals/collector/otel-collector-config.yaml for the
// adaptive_tail_sampling rules this dataset is built to exercise.
//
// Run this against the Collector from session-1-fundamentals/collector, same
// as every other session. Unlike session 4, there's no "reseed shortly before
// you go live" requirement — this dataset has no incident window to go
// stale, just a point-in-time cost comparison.
package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strconv"
	"time"

	"github.com/honeycombio/o11y-eng-masterclass/session-5-cost-sampling/internal/scenario"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

const serviceName = "cost-sampling-service"

// maxQueueSize is the BatchSpanProcessor queue depth. Named rather than
// inlined because the delivery test needs it: a test that seeds fewer spans
// than this cannot detect a dropped-span regression, so it asserts against
// this value rather than a hardcoded guess that would silently rot if this
// changed. Same rationale as session 4's constant of the same name.
const maxQueueSize = 8192

var tracer = otel.Tracer(serviceName)

func main() {
	ctx := context.Background()

	provider, err := setupTracing(ctx)
	if err != nil {
		log.Fatalf("setting up tracing: %v", err)
	}

	cfg := scenario.DefaultConfig()
	if v := os.Getenv("SEED_COUNT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.RequestCount = n
		}
	}

	// How long after the last span is flushed to wait before shutting down.
	// The real Collector's adaptive_tail_sampling processor buffers each
	// trace for decision_delay (2s) past its root span before deciding, and
	// the adaptive_percentage sampler's goal_percentage rate only settles in
	// after adjustment_interval (15s) has seen enough traces to adapt. Exit
	// immediately after flushing and you'd race the Collector's own decision
	// window. The delivery test overrides this to near-zero: its in-process
	// receiver never samples, so there's nothing to wait for.
	settleWait := 15 * time.Second
	if v := os.Getenv("SEED_SETTLE_WAIT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			settleWait = d
		}
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	requests := scenario.Generate(cfg, rng)

	// Emit in chunks, flushing after each — same reasoning as session 4's
	// seeder: the BatchSpanProcessor silently drops anything that overflows
	// its queue, and this dataset emits two spans per request rather than
	// one, so the queue fills twice as fast for the same request count.
	const chunk = 500

	var errored int
	for i, req := range requests {
		emit(ctx, req)
		if req.Errored {
			errored++
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

	errorRate := float64(errored) / float64(len(requests))
	// goalPercentage matches the Collector's cost-sampling-service rule
	// (see the config referenced in the package comment). Duplicated here as
	// a literal, not imported, because this print is illustrative context
	// for the person running the seeder, not a correctness dependency; the
	// real number comes from the Collector at query time.
	const goalPercentage = 0.05
	estimatedRetained := errorRate + (1-errorRate)*goalPercentage

	fmt.Printf(`
seeded %d requests (%d spans) into service %q
  synthetic error rate   %.1f%% (%d requests) — the Collector keeps all of these
  everything else        sampled to ~%.0f%% by the Collector's cost-sampling-service rule

Estimated retained share once the Collector has decided: %.1f%% (~%.0f%% reduction).
That's an estimate from this run's own numbers, not a substitute for querying
Honeycomb — see ../../honeycomb-setup for the weighted-COUNT query that shows
the real number.

Waiting %s for the Collector's decision_delay and adjustment_interval to
settle before exiting...
`,
		len(requests), len(requests)*2, serviceName,
		errorRate*100, errored,
		goalPercentage*100,
		estimatedRetained*100, (1-estimatedRetained)*100,
		settleWait,
	)

	time.Sleep(settleWait)

	if err := provider.Shutdown(ctx); err != nil {
		log.Fatalf("shutting down tracer provider: %v", err)
	}
}

// flush blocks until the exporter has shipped everything queued so far, so
// the caller can safely generate the next chunk.
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
	// plaintext, never straight to Honeycomb; the Collector holds the API
	// key and, for this session, the sampling decision.
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

// emit turns one generated request into a two-span trace: a root HTTP span
// and one db.query child. Two spans, not one, because the Collector's
// adaptive_tail_sampling processor decides per whole trace — a dataset of
// single-span traces couldn't demonstrate that a trace is kept or dropped as
// a unit.
func emit(ctx context.Context, req scenario.Request) {
	rootCtx, root := tracer.Start(ctx, fmt.Sprintf("POST %s", req.Route))
	root.SetAttributes(
		semconv.HTTPRequestMethodPost,
		semconv.HTTPRoute(req.Route),
		attribute.String("service.environment", "production"),
	)

	_, child := tracer.Start(rootCtx, "db.query")
	child.SetAttributes(attribute.String("db.system", "postgresql"))

	if req.Errored {
		for _, span := range []trace.Span{root, child} {
			span.SetStatus(codes.Error, req.ErrorType)
			span.SetAttributes(attribute.Bool("error", true), attribute.String("error.type", req.ErrorType))
		}
	}

	child.End()
	root.End()
}
