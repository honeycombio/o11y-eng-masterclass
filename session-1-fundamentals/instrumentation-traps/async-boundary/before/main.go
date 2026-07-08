// Trap: a producer starts a span, then hands work to a goroutine (standing
// in for a Kafka consumer, background job, or cron run) via a channel that
// only carries the payload. The consumer has no way to know a trace was
// ever in progress, so its span starts a disconnected new trace.
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

const serviceName = "instrumentation-traps-async-boundary-before"

var tracer = otel.Tracer(serviceName)

// message is what crosses the queue boundary — payload only.
type message struct {
	orderID string
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
	queue <- message{orderID: "ord_42"}
	produceSpan.End()

	<-done
	fmt.Println("BUG: the queue only carried the payload, so the consumer's span is a disconnected new trace.")

	flushCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := shutdown(flushCtx); err != nil {
		log.Fatalf("flushing spans (is the Collector up? see ../../../collector): %v", err)
	}
}

func consume(queue <-chan message, done chan<- struct{}) {
	msg := <-queue

	ctx, span := tracer.Start(context.Background(), "consume_order_message")
	defer span.End()

	fmt.Printf("consumer trace ID:  %s (different from producer!) order=%s\n", trace.SpanContextFromContext(ctx).TraceID(), msg.orderID)
	time.Sleep(5 * time.Millisecond)
	done <- struct{}{}
}
