// Package vendorlib is a stand-in for a third-party dependency (an ORM, an
// HTTP client, an SDK) that instruments its own internals heavily. It's not
// part of the trap or the fix — both sides of the demo use it unchanged; only
// the TracerProvider handed to NewClient differs.
package vendorlib

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/trace"
)

type Client struct {
	tracer trace.Tracer
}

// NewClient takes the TracerProvider the caller wants this library's spans
// to go through — a real one to see them, a noop one to silence them.
func NewClient(tp trace.TracerProvider) *Client {
	return &Client{tracer: tp.Tracer("vendor-library-x")}
}

// Do simulates a call that, internally, is four spans' worth of plumbing
// most callers never need to see.
func (c *Client) Do(ctx context.Context) {
	for _, step := range []struct {
		name string
		cost time.Duration
	}{
		{"acquire_connection", 2 * time.Millisecond},
		{"serialize", time.Millisecond},
		{"network_write", 3 * time.Millisecond},
		{"network_read", 3 * time.Millisecond},
	} {
		_, span := c.tracer.Start(ctx, step.name)
		time.Sleep(step.cost)
		span.End()
	}
}
