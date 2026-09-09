// seed-llm-eval-pipeline backfills the Masterclass 6 LLM-eval supplement: a
// batch of scored conversations standing in for the output of an eval
// pipeline. See internal/llmscenario for why this dataset is small and
// narrow — the live LLM demo runs on real Claude Code OTel telemetry (see
// ../../README.md) for everything except quality scoring, which real
// telemetry has no signal for at all.
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

	"github.com/honeycombio/o11y-eng-masterclass/session-6-every-domain/internal/llmscenario"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.41.0"
	"go.opentelemetry.io/otel/trace"
)

const serviceName = "llm-eval-pipeline"

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

	cfg := llmscenario.DefaultConfig()
	cfg.Now = time.Now()
	if v := os.Getenv("SEED_COUNT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.ConversationCount = n
		}
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	convos := llmscenario.Generate(cfg, rng)

	const chunk = 1000

	var regressed int
	for i, c := range convos {
		emit(ctx, c, rng)
		if c.Regressed {
			regressed++
		}
		if (i+1)%chunk == 0 {
			if err := flush(ctx, provider); err != nil {
				log.Fatalf("flushing spans after %d conversations: %v", i+1, err)
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
seeded %d scored conversations into service %q
  quality-regressed  %d (%.0f%%) — every one still reads as a normal, successful conversation otherwise

See ../../honeycomb-setup for the saved SLI query (MC4's own formula:
count(llm.response.quality_score > 0.7) / count(*)), reused unmodified here.
`,
		len(convos), serviceName,
		regressed, 100*float64(regressed)/float64(len(convos)),
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

// evalDuration is the nominal time the (real, out-of-band) eval pipeline
// takes to score one conversation. Not load-bearing for the demo query, but
// a real span shouldn't read as zero-duration when someone clicks into it.
const evalDurationBase = 200 * time.Millisecond

// emit turns one scored conversation into a single root span at an explicit
// historical timestamp — this dataset is eval-pipeline output, not a live
// trace, so there is nothing a child span would add.
func emit(ctx context.Context, c llmscenario.Conversation, rng *rand.Rand) {
	end := c.Start.Add(evalDurationBase + time.Duration(rng.Intn(300))*time.Millisecond)
	_, span := tracer.Start(ctx, "scored conversation", trace.WithTimestamp(c.Start))
	span.SetAttributes(
		attribute.Float64("llm.response.quality_score", c.QualityScore),
	)
	span.End(trace.WithTimestamp(end))
}
