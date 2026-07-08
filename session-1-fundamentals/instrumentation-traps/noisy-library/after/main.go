// Fix: vendorlib.NewClient is handed a noop TracerProvider instead of the
// real one, so its internal spans are dropped at the source — the app's own
// tracer (still bound to the real global provider) is unaffected.
package main

import (
	"context"
	"log"
	"time"

	"github.com/honeycombio/o11y-eng-masterclass/session-1-fundamentals/instrumentation-traps/internal/demotrace"
	"github.com/honeycombio/o11y-eng-masterclass/session-1-fundamentals/instrumentation-traps/noisy-library/vendorlib"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace/noop"
)

const serviceName = "instrumentation-traps-noisy-library-after"

var tracer = otel.Tracer(serviceName)

func main() {
	ctx := context.Background()

	shutdown, err := demotrace.Setup(ctx, serviceName)
	if err != nil {
		log.Fatalf("setting up tracing: %v", err)
	}

	// FIXED: the noop provider is the "escape hatch" from the slides — this
	// dependency's spans never get created, so they can't flood the trace.
	client := vendorlib.NewClient(noop.NewTracerProvider())

	ctx, span := tracer.Start(ctx, "handle_request")
	client.Do(ctx)
	span.End()

	log.Println("FIXED: this trace has 1 span (handle_request); vendorlib's internals never got created.")

	flushCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := shutdown(flushCtx); err != nil {
		log.Fatalf("flushing spans (is the Collector up? see ../../../collector): %v", err)
	}
}
