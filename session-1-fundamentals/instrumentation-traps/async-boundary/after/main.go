// Fix: the producer injects the active trace context into the message's
// carrier (a map[string]string riding alongside the payload — the "baggage"
// the slide refers to). The consumer extracts it back into a context before
// starting its span, so the span joins the producer's trace.
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/honeycombio/o11y-eng-masterclass/session-1-fundamentals/instrumentation-traps/internal/demotrace"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

const serviceName = "instrumentation-traps-async-boundary-after"

var tracer = otel.Tracer(serviceName)

// message carries an explicit trace-context carrier alongside the payload.
type message struct {
	orderID string
	carrier propagation.MapCarrier
}

func main() {
	ctx := context.Background()

	shutdown, err := demotrace.Setup(ctx, serviceName)
	if err != nil {
		log.Fatalf("setting up tracing: %v", err)
	}

	queue := make(chan message, 1)
	done := make(chan struct{})
	go consume(queue, done)

	ctx, produceSpan := tracer.Start(ctx, "produce_order_message")
	fmt.Printf("producer trace ID: %s\n", trace.SpanContextFromContext(ctx).TraceID())

	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	queue <- message{orderID: "ord_42", carrier: carrier}
	produceSpan.End()

	<-done
	fmt.Println("FIXED: the carrier crossed the queue boundary, so the consumer's span joins the producer's trace.")

	flushCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := shutdown(flushCtx); err != nil {
		log.Fatalf("flushing spans (is the Collector up? see ../../../collector): %v", err)
	}
}

func consume(queue <-chan message, done chan<- struct{}) {
	msg := <-queue

	ctx := otel.GetTextMapPropagator().Extract(context.Background(), msg.carrier)
	ctx, span := tracer.Start(ctx, "consume_order_message")
	defer span.End()

	fmt.Printf("consumer trace ID:  %s (same as producer) order=%s\n", trace.SpanContextFromContext(ctx).TraceID(), msg.orderID)
	time.Sleep(5 * time.Millisecond)
	done <- struct{}{}
}
