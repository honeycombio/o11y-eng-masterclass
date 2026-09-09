// seed-cicd-pipeline backfills the Masterclass 6 CI/CD dataset: a pipeline
// run many times over a two-week window, with a step-change build-duration
// regression in the trailing day and one intermittently failing test case.
// See internal/cicdscenario for the scenario and why its proportions are
// what they are.
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

	"github.com/honeycombio/o11y-eng-masterclass/session-6-every-domain/internal/cicdscenario"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.41.0"
	"go.opentelemetry.io/otel/trace"
)

const serviceName = "cicd-pipeline"

// maxQueueSize is the BatchSpanProcessor queue depth. Named rather than
// inlined because the delivery test needs it — same rationale as sessions 4
// and 5's constant of the same name.
const maxQueueSize = 8192

var tracer = otel.Tracer(serviceName)

func main() {
	ctx := context.Background()

	provider, err := setupTracing(ctx)
	if err != nil {
		log.Fatalf("setting up tracing: %v", err)
	}

	cfg := cicdscenario.DefaultConfig()
	cfg.Now = time.Now()
	if v := os.Getenv("SEED_COUNT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.RunCount = n
		}
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	runs := cicdscenario.Generate(cfg, rng)

	// Emit in chunks, flushing after each — same reasoning as every other
	// seeder in this repo: the BatchSpanProcessor silently drops anything
	// that overflows its queue, and this dataset emits many spans per run.
	const chunk = 50

	var regressed, failed int
	for i, run := range runs {
		emitRun(ctx, run)
		if run.Regressed {
			regressed++
		}
		if run.Result == cicdscenario.ResultFailure {
			failed++
		}
		if (i+1)%chunk == 0 {
			if err := flush(ctx, provider); err != nil {
				log.Fatalf("flushing spans after %d runs: %v", i+1, err)
			}
		}
	}

	if err := flush(ctx, provider); err != nil {
		log.Fatalf("final flush: %v", err)
	}
	if err := provider.Shutdown(ctx); err != nil {
		log.Fatalf("shutting down tracer provider: %v", err)
	}

	regressionStart := cfg.RegressionStart()
	fmt.Printf(`
seeded %d pipeline runs into service %q
  build regression   started %s, still ongoing at seed time
  regressed runs      %d
  flaky test failures %d (out of %d runs) on %s

The regression is anchored relative to now, so a trailing-window P95 trigger
on the build task only finds it if you seed shortly before you go live.
`,
		len(runs), serviceName,
		regressionStart.Format(time.RFC3339),
		regressed,
		failed, len(runs), cicdscenario.FlakyTestName,
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

// emitRun turns one generated run into a trace: a root pipeline span, four
// task spans as its children, and the test task's test cases as its own
// children in turn. Every span gets an explicit historical timestamp so the
// whole two-week window can be backfilled in one pass, same as sessions 4's
// seeder.
func emitRun(ctx context.Context, run cicdscenario.Run) {
	end := run.Start
	for _, t := range run.Tasks {
		end = end.Add(t.Duration)
	}

	runCtx, root := tracer.Start(ctx, "pipeline run", trace.WithTimestamp(run.Start))
	root.SetAttributes(
		semconv.CICDPipelineName("masterclass-app-ci"),
		attribute.String("cicd.pipeline.run.id", run.RunID),
		semconv.VCSChangeID(strconv.Itoa(run.PRNumber)),
		semconv.VCSRefHeadName(run.Branch),
	)
	if run.Result == cicdscenario.ResultFailure {
		root.SetAttributes(semconv.CICDPipelineResultFailure)
		root.SetStatus(codes.Error, "pipeline failed")
	} else {
		root.SetAttributes(semconv.CICDPipelineResultSuccess)
	}

	taskStart := run.Start
	for _, task := range run.Tasks {
		emitTask(runCtx, task, taskStart)
		taskStart = taskStart.Add(task.Duration)
	}

	root.End(trace.WithTimestamp(end))
}

func emitTask(ctx context.Context, task cicdscenario.Task, start time.Time) {
	taskCtx, span := tracer.Start(ctx, task.Name, trace.WithTimestamp(start))
	span.SetAttributes(semconv.CICDPipelineTaskName(task.Name))
	if task.Result == cicdscenario.ResultFailure {
		span.SetAttributes(semconv.CICDPipelineTaskRunResultFailure)
		span.SetStatus(codes.Error, "task failed")
	} else {
		span.SetAttributes(semconv.CICDPipelineTaskRunResultSuccess)
	}

	if len(task.Tests) > 0 {
		testStart := start
		testDuration := task.Duration / time.Duration(len(task.Tests))
		for _, tc := range task.Tests {
			emitTestCase(taskCtx, tc, testStart, testDuration)
			testStart = testStart.Add(testDuration)
		}
	}

	span.End(trace.WithTimestamp(start.Add(task.Duration)))
}

func emitTestCase(ctx context.Context, tc cicdscenario.TestCase, start time.Time, duration time.Duration) {
	_, span := tracer.Start(ctx, tc.Name, trace.WithTimestamp(start))
	span.SetAttributes(attribute.String("test.name", tc.Name))
	if tc.Result == cicdscenario.ResultFailure {
		span.SetAttributes(attribute.Bool("error", true))
		span.SetStatus(codes.Error, "test failed")
	}
	span.End(trace.WithTimestamp(start.Add(duration)))
}
