# otel-quickstart

The Masterclass 1 live demo: "auto-instrumentation → custom span." A tiny
checkout service, auto-instrumented with
[`otelhttp`](https://pkg.go.dev/go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp),
that gets one hand-written span added for the part auto-instrumentation can't
see.

## Pre-session setup (do this before you're live)

1. From `../collector`, start the local Collector:

   ```bash
   HONEYCOMB_API_KEY=<your key> docker compose up -d
   ```

2. In this directory: `go run main.go`
3. Send one request and confirm a span lands in Honeycomb under the
   `otel-quickstart` dataset:

   ```bash
   curl -X POST http://localhost:8080/api/checkout
   ```

4. Rehearse the live-edit below once so the diff is muscle memory, not
   something you're composing on stage.

## Live demo script

1. **Show auto-instrumentation alone.** Comment out the block between
   `--- CUSTOM SPAN ---` and `--- END CUSTOM SPAN ---` in `main.go` (leave
   `simulateOrderProcessing(ctx)` uncommented). Restart the server, send a
   request, show the trace in Honeycomb: one span, the HTTP request itself,
   courtesy of `otelhttp` — zero application code involved.
2. **Add the custom span.** Uncomment the block back in:

   ```go
   ctx, span := tracer.Start(ctx, "process_order")
   span.SetAttributes(attribute.String("order.id", orderID))
   defer span.End()
   ```

   Restart, send another request, and query `order.id` in Honeycomb to
   confirm the new span landed as a child of the HTTP span.

## Async lab

Same two steps, at your own pace, plus:

- Add a second attribute to the custom span from a different one of the four
  attribute categories from the slides (identity, request/execution, outcome,
  service/code context).
- Stretch goal: ask an AI assistant to instrument a second endpoint you add
  yourself, then review its attribute choices before accepting them — treat
  it like a PR from a junior engineer.

## What auto-instrumentation actually gives you

Verified on the wire (OTLP), `otelhttp` v0.69 puts 12 attributes on the server
span for free, with zero application code:

`client.address`, `http.request.method`, `http.response.body.size`,
`http.response.status_code`, `network.peer.address`, `network.peer.port`,
`network.protocol.version`, `server.address`, `server.port`, `url.path`,
`url.scheme`, `user_agent.original`

Notably **absent**: `http.route`. This version of `otelhttp` has no
`WithRouteTag` option, so the matched route template never lands on its own —
`setRoute()` in `main.go` adds it explicitly. Worth calling out live: it's a
real instance of Chapter 6's "don't settle for only what your instrumentation
library gives you by default," and `http.route` is what Masterclass 4's SLI
filters on.

See [`../instrumentation-traps/`](../instrumentation-traps) for the
context-propagation and noisy-instrumentation traps mentioned in the slides;
those aren't part of this service, since they trigger on situations
(goroutines, third-party libraries) it doesn't have.
