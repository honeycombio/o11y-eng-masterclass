# Session 1: Fundamentals — Wide Events & Instrumentation with OTel

Code accompanying Masterclass 1. See [`../masterclass-curriculum-45min.md`](../masterclass-curriculum-45min.md)
for the full session outline; this directory has everything the live demo
and the async lab reference.

## Layout

- [`collector/`](collector) — local OTel Collector config + docker-compose,
  shared by everything below (including session 5). Start this first. Runs
  Honeycomb's own Collector distro rather than the plain contrib image, since
  session 5's cost/sampling demo needs its `adaptive_tail_sampling` processor;
  every other session's traffic falls through that config's default rule
  untouched — see [`../session-5-cost-sampling/README.md`](../session-5-cost-sampling/README.md).
- [`otel-quickstart/`](otel-quickstart) — the live demo: auto-instrumentation
  via `otelhttp`, then one hand-written custom span.
- [`seed-sample-service/`](seed-sample-service) — pre-session seeder for the
  `sample-service` dataset used in the "one dataset, three ways" demo.
- [`instrumentation-traps/`](instrumentation-traps) — async-lab only: the
  three instrumentation traps from the slides, as before/after pairs.

This is a Go workspace (`go.work`) covering all three Go modules, so
`gopls`/editor tooling works across directories without extra setup.

## Pre-session checklist (do this before you're live)

1. `HONEYCOMB_API_KEY=<your key> docker compose -f collector/docker-compose.yml up -d`
2. `cd seed-sample-service && go run main.go` — backfills `sample-service`
   with ~400 checkout traces so the "one dataset, three ways" demo has real
   data to query.
3. `cd otel-quickstart && go run main.go` — confirm it's serving, send one
   request, confirm a span lands under the `otel-quickstart` dataset.
4. Rehearse the `otel-quickstart` live-edit once (see its README) so it's
   muscle memory, not composed on stage.

Nothing here is built from scratch live — steps 1–3 happen before the
session; the ~45 minutes live is running and querying what's already set up.

### If the Collector won't start

The image is pinned to a known-good version, so the usual causes are local:

- **`address already in use`** — something already holds 4317 or 4318, often a
  Collector left running from an earlier attempt. `docker compose ps` and
  `lsof -i :4317`, then `docker compose down` before retrying.
- **`HONEYCOMB_API_KEY` unset** — Compose fails fast and tells you. If you
  bypass Compose with a bare `docker run`, you instead get a confusing config
  error from the exporter's `${env:HONEYCOMB_API_KEY}` lookup, so prefer
  Compose.
- **401 `unknown API key`** in the Collector logs — the Collector is fine and
  your spans reached it; the key is wrong, or it's for the wrong region.
  Note this is only visible in the Collector's own logs, not in your app's.

## Live demo run order

1. **One dataset, three ways** (`seed-sample-service`'s data): count/rate,
   filtered search `WHERE error = true`, trace waterfall.
2. **Auto, attribute, then span** (`otel-quickstart`), in three beats: show
   `otelhttp` alone; add `order.id` to the span it already created; then add a
   `process_order` span, once there's a duration worth measuring on its own.
   Beats 2 and 3 ship commented out, so `git restore` resets the demo.

## Async lab (self-serve, ~2 hours — not time-boxed to the live 45 min)

1. Reconstruct one trace three ways using the seeded `sample-service` data.
2. Clone `otel-quickstart`, confirm auto-instrumented spans arrive, add the
   custom span yourself, verify by querying `order.id`.
3. Work through `instrumentation-traps/`: all three before/after pairs.
4. Stretch goal: have an AI assistant scaffold instrumentation for a second
   endpoint on `otel-quickstart`, then review its attribute choices against
   this session's four categories (identity, request/execution, outcome,
   service/code context) before accepting them.

This same material also works as raw content for a longer instructor-led
workshop format — it isn't written assuming Liz is narrating it live, so
steps 2–4 stand on their own.
