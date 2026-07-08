# instrumentation-traps

Async-lab material, not part of the 45-minute live session. The slides call
out three traps that eat the most instrumentation debugging time; each gets
its own before/after pair here so you can see the broken trace and the
fixed one side by side.

| Trap | Directory | The bug |
|------|-----------|---------|
| Missing `context.Context` propagation | [`context-propagation/`](context-propagation) | A helper function doesn't accept a `context.Context`, so its span starts a new trace instead of joining the caller's. |
| Async boundaries drop context | [`async-boundary/`](async-boundary) | Work handed to a goroutine (standing in for a Kafka consumer, background job, or cron run) carries only the payload, not the trace context. |
| Noisy library instrumentation | [`noisy-library/`](noisy-library) | A dependency's own spans flood every trace; fixed with the `noop` TracerProvider escape hatch, scoped to just that dependency. |

## Running one

Each variant is a standalone `main.go`:

```bash
cd context-propagation/before && go run main.go
cd ../after && go run main.go
```

Every variant prints the trace ID(s) involved directly to stdout, so you can
see the bug (or the fix) without needing Honeycomb open — though if the
local Collector (see [`../collector`](../collector)) is running, the same
traces land there too, and you can confirm visually that the "before" spans
are split across separate traces (or, for the noisy-library trap, that the
"before" trace has 5 spans and the "after" trace has 1).

## Lab exercise

For each pair:

1. Run `before`, note what's broken (differing trace IDs, or an extra 4
   spans).
2. Read the diff between `before/main.go` and `after/main.go` — each is
   under 15 lines different.
3. Run `after`, confirm the fix.

Then, in your own service: pick one function that takes no `context.Context`
today and would need one to be traced correctly. Add the parameter and wire
it through.

## Going deeper on async-boundary

The `async-boundary` fix here uses direct parent-child spans (via context
propagation across the queue) because it's the simplest correct fix for a
single background job or cron run, and matches what the slide describes:
"propagate baggage explicitly." For a real high-throughput queue like Kafka,
the book (Chapter 7, "Tracing streaming architectures") recommends something
more nuanced: producer/consumer spans joined by **span links** rather than
direct parent-child, precisely because forcing every consumer into one
literal trace tree causes a "million children trace" problem at scale. If
you're instrumenting an actual message queue rather than a single background
job, read that section before copying this pattern as-is.
