// Trap: vendorlib.NewClient is handed the real, global TracerProvider, so
// its four internal spans per call land in every trace right alongside your
// own — the "noisy library instrumentation floods traces" problem.
package main

import (
	"context"
	"log"
	"time"

	"github.com/honeycombio/o11y-eng-masterclass/session-1-fundamentals/instrumentation-traps/internal/demotrace"
	"github.com/honeycombio/o11y-eng-masterclass/session-1-fundamentals/instrumentation-traps/noisy-library/vendorlib"
	"go.opentelemetry.io/otel"
)

const serviceName = "instrumentation-traps-noisy-library-before"

var tracer = otel.Tracer(serviceName)

func main() {
	ctx := context.Background()

	shutdown, err := demotrace.Setup(ctx, serviceName)
	if err != nil {
		log.Fatalf("setting up tracing: %v", err)
	}

	// BUG: the real provider means every vendorlib.Do() call adds 4 spans
	// (acquire_connection, serialize, network_write, network_read) to this
	// trace that nobody debugging *this* service needs to see.
	client := vendorlib.NewClient(otel.GetTracerProvider())

	ctx, span := tracer.Start(ctx, "handle_request")
	client.Do(ctx)
	span.End()

	log.Println("BUG: this trace has 5 spans (handle_request + 4 noisy vendorlib internals).")

	flushCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := shutdown(flushCtx); err != nil {
		log.Fatalf("flushing spans (is the Collector up? see ../../../collector): %v", err)
	}
}
