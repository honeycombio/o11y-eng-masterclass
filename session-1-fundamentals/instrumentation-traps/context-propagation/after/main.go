// Fix: validateOrder takes a context.Context and passes it to tracer.Start,
// so its span becomes a child of whatever span is already active in ctx.
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/honeycombio/o11y-eng-masterclass/session-1-fundamentals/instrumentation-traps/internal/demotrace"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

const serviceName = "instrumentation-traps-context-propagation-after"

var tracer = otel.Tracer(serviceName)

func main() {
	ctx := context.Background()

	shutdown, err := demotrace.Setup(ctx, serviceName)
	if err != nil {
		log.Fatalf("setting up tracing: %v", err)
	}

	ctx, root := tracer.Start(ctx, "handle_request")
	fmt.Printf("handle_request trace ID: %s\n", trace.SpanContextFromContext(ctx).TraceID())
	validateOrder(ctx)
	root.End()

	fmt.Println("FIXED: validateOrder takes ctx and passes it to tracer.Start, so it joins the same trace.")

	flushCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := shutdown(flushCtx); err != nil {
		log.Fatalf("flushing spans (is the Collector up? see ../../../collector): %v", err)
	}
}

// validateOrder takes the caller's context.Context, so its span is a child
// of whatever span is active when it's called.
func validateOrder(ctx context.Context) {
	ctx, span := tracer.Start(ctx, "validate_order")
	defer span.End()

	fmt.Printf("validate_order trace ID: %s (same as handle_request)\n", trace.SpanContextFromContext(ctx).TraceID())
	time.Sleep(10 * time.Millisecond)
}
