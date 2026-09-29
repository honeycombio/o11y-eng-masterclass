# Session 5: What Observability Costs, and How to Make It Cost Less

Code accompanying Masterclass 5. See
[`../masterclass-curriculum-45min.md`](../masterclass-curriculum-45min.md) for
the session outline.

**Book mapping:** Chapter 13 (Efficient Data Storage with Retriever), Chapter
14 (Efficient Data Storage with ClickHouse), Chapter 15 (Cheap and Accurate
Enough Sampling), Chapter 16 (Telemetry Management with Pipelines), Chapter 27
(Diagnosing Your Observability Investment), Chapter 29 (Build Versus Buy
(Versus Open Source)), Chapter 30 (The Art and Science of Vendor Partnerships).

This session's live demo covers Chapter 15's sampling section only — the
curriculum's other MC5 material (storage engines, pipelines, vendor
partnerships) is slides-only, same as MC4 illustrated three of its four SLI
formulas without separately seeding them.

## The Collector processor this demo depends on, read this first

The sampling demo is real tail sampling — `keep every error, sample the rest
by key at a target rate` — running as a single processor
(`adaptive_tail_sampling`) inside session-1's shared Collector, rather than a
separate Refinery deployment. It's Honeycomb's own build of the upstream
`adaptivetailsamplingprocessor`, which is explicitly `development` stability:
"do not rely on it for critical workloads yet," per the distro's own README.
That's fine for a workshop demo whose entire dataset is disposable
synthetic traffic, and it was verified empirically (a local run against a
real Honeycomb dataset, confirming the sample rate — recorded as W3C
tracestate `ot=th:<hex>`, not the older `SampleRate` integer attribute — comes
back out the other side as a correctly weighted `COUNT()`) before this session
was built. If Honeycomb bumps the distro version and something here breaks,
that warning is why: pin bumps to this processor need re-verifying, not just
re-running.

## Layout

- [`cmd/seed-cost-sampling-service/`](cmd/seed-cost-sampling-service) — the
  demo dataset, emitted in real time (see below).
- [`internal/scenario/`](internal/scenario) — the scenario itself, with the
  tuning arithmetic documented and tested.
- [`honeycomb-setup/`](honeycomb-setup) — two saved queries, as Terraform. No
  free-tier gate here at all — see its README.

The Collector from
[`../session-1-fundamentals/collector`](../session-1-fundamentals/collector)
is shared with every other session, but session 5 is the reason it now runs
Honeycomb's distro image with the `adaptive_tail_sampling` processor added —
see that directory's `otel-collector-config.yaml` for the rules. MC1-4's
traffic falls through this config's default `always_sample` rule untouched.

## The scenario

One service, `cost-sampling-service`, four routes at a skewed 85/10/4/1
distribution, and a synthetic 15% error rate. Every request is a two-span
trace (root HTTP span + one `db.query` child) — the Collector decides
per whole trace, so single-span traces couldn't show that. Unlike session 4,
nothing is backdated: spans are emitted at real wall-clock time, paced evenly
over 90 seconds, because the Collector's decision window (`decision_delay`,
`adjustment_interval`) only means something against real arrival timing.

The Collector's `cost-sampling-service` rule keeps every errored trace and
samples the rest to a `goal_percentage` of 5%, fingerprinted on `http.route`
so each route's rate adapts independently rather than one global rate
swallowing the rarest route (`/api/admin/report`, 1% of traffic) into
statistical noise. See
[`internal/scenario/scenario.go`](internal/scenario/scenario.go)'s package
comment for the exact retained-share arithmetic.

The pacing is load-bearing. The per-route sampler has no per-route rates
until its first 15s `adjustment_interval` tick, and until then samples every
route at the flat goal rate. A burst seed would land entirely inside that
first interval and demonstrate a flat 1-in-20 sample, not per-route
sampling. The sampler also carries state between runs, so the checklist
restarts the Collector before each seed.

## Pre-session checklist

1. Apply the Terraform (two saved queries) — see
   [`honeycomb-setup/`](honeycomb-setup). Any time beforehand.

2. Within 30 minutes of going live (the saved queries' window), restart the
   Collector (Honeycomb's distro image, not plain contrib — see
   [`../session-1-fundamentals/collector`](../session-1-fundamentals/collector))
   so the sampler starts cold, then seed once:

   ```bash
   cd ../session-1-fundamentals/collector
   HONEYCOMB_API_KEY=<send-events key> docker compose up -d --force-recreate
   cd ../../session-5-cost-sampling
   go run ./cmd/seed-cost-sampling-service
   ```

   The seeder takes about two minutes (90s of paced emission, then 15s for
   the Collector to settle). Seed only once inside the window: a second run
   doubles every count.

3. Confirm in Honeycomb: `cost_sampling_population`'s `COUNT()` reads close
   to the seeder's own printed request count, and `cost_sampling_by_route`
   shows all four routes, including `/api/admin/report` at roughly its seeded
   1% share.

4. Confirm in Usage Mode (Usage page → Per-environment Breakdown → Usage
   Mode, where `COUNT()` is unweighted and `Sample Rate` is a queryable
   field): `COUNT()` is roughly 19% of the seeded request count, and
   `COUNT()` broken down by `http.route` and `Sample Rate` shows
   `/api/admin/report`'s healthy traffic kept at a much lower sample rate
   than `/api/search`'s. Expect three kinds of row per route: sample rate 1
   (errors, via `keep-errors`), 20 (the first 15s, before the sampler has
   per-route rates), and the per-route adapted rate. If the adapted rows are
   missing, the per-route sampler didn't engage — check the Collector was
   restarted and `SEED_DURATION` wasn't shortened.

## Live demo run order

**Where the money goes** (slides only, 7 min): ingest volume, retention,
query compute — no live component.

**Sampling, the biggest lever** (7 min slides, then 10 min live):

1. Show the Collector's `adaptive_tail_sampling` rules in
   `otel-collector-config.yaml` — `keep-errors` ahead of the
   `cost-sampling-service` catch-all, ahead of `default`.
2. Run `cost_sampling_population`: the weighted `COUNT()` reads close to what
   the seeder printed.
3. Switch to Usage Mode and run the same `COUNT()` unweighted: about a fifth
   of that number is what's physically stored. The gap between the two,
   reconciled automatically from the recorded sample rate, is the whole demo.
4. Run `cost_sampling_by_route`: all four routes are visible in roughly their
   seeded proportions, including the 1% route.
5. In Usage Mode, break `COUNT()` down by `http.route` and `Sample Rate`: the
   sampler kept `/api/admin/report`'s healthy traffic at a far lower rate
   than `/api/search`'s. That's the payoff of fingerprinting on `http.route`
   instead of applying one flat rate, shown directly rather than inferred
   from step 4 (which a flat rate plus keep-all-errors would also pass).
6. Land on the dollar arithmetic in the curriculum: keep-all-errors (15%) plus
   5%-of-the-rest retains ~19% of events, an ~81% reduction — a $50k/month
   bill becomes roughly $9.5k/month with no meaningful loss of debugging
   power, because every error is still there. The 15% error rate is
   deliberately inflated for the demo; at a realistic sub-1% error rate the
   same rules retain ~6%, a ~94% reduction.

**Pipelines and storage / organisational discipline** (slides only, 18 min):
no live component in this repo.

## Async lab

1. Pull your own team's usage data and rank your top three cost drivers —
   which datasets grow fastest, which services contribute most volume.
2. Model one sampling change against this seeded dataset: try a different
   `goal_percentage` in the Collector config (restart the Collector, reseed,
   requery in both normal and Usage Mode) and recompute the retained-share
   arithmetic from `internal/scenario`'s package comment to match.
3. Bring the number to your next budget conversation.

## Tuning

`SEED_COUNT` (5000 requests, 10000 spans), `SEED_DURATION` (90s — how long
emission is paced over; must span several of the Collector's
`adjustment_interval`s, which `TestDefaultConfig_DurationSpansAdjustmentIntervals`
enforces against the real Collector config), `SEED_SETTLE_WAIT` (15s — how
long the seeder waits after its last flush for the Collector's
`decision_delay` and `adjustment_interval` to settle before exiting). The
delivery test overrides both durations to near-zero since its in-process
receiver never samples. Route shares and the error rate are in
[`internal/scenario/scenario.go`](internal/scenario/scenario.go)'s
`DefaultConfig` — the tests enforce the bounds that keep the demo working.
