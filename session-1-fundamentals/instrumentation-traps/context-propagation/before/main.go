// Trap: validateOrder doesn't accept a context.Context, so it has no parent
// to attach to and starts a brand-new trace instead of a child span. The
// span you added "never appears in the trace" — it appears in its own trace.
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

const serviceName = "instrumentation-traps-context-propagation-before"

var tracer = otel.Tracer(serviceName)

func main() {
	ctx := context.Background()

	shutdown, err := demotrace.Setup(ctx, serviceName)
	if err != nil {
		log.Fatalf("setting up tracing: %v", err)
	}

	ctx, root := tracer.Start(ctx, "handle_request")
	fmt.Printf("handle_request trace ID: %s\n", trace.SpanContextFromContext(ctx).TraceID())
	validateOrder()
	root.End()

	fmt.Println("BUG: validateOrder has no ctx parameter, so its span starts a new trace instead of joining this one.")

	flushCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := shutdown(flushCtx); err != nil {
		log.Fatalf("flushing spans (is the Collector up? see ../../../collector): %v", err)
	}
}

// validateOrder takes no context.Context, so it has nothing to derive a
// child span from.
func validateOrder() {
	ctx, span := tracer.Start(context.Background(), "validate_order")
	defer span.End()

	fmt.Printf("validate_order trace ID: %s (different from handle_request!)\n", trace.SpanContextFromContext(ctx).TraceID())
	time.Sleep(10 * time.Millisecond)
}
