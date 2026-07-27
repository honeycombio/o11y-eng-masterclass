# Session 2: Feedback Loops — Observability Connects Code to Delivery

Code accompanying Masterclass 2. See
[`../masterclass-curriculum-45min.md`](../masterclass-curriculum-45min.md) for
the session outline.

**Book mapping:** Chapter 2 (How Code Crosses Over), Chapter 8 (Getting Started
with Observability Analysis), Chapter 9 (Observability-Driven Development),
Chapter 10 (The Role of AI Agents for Observability), Chapter 24 (Systems
Thinking for Software Delivery).

## Layout

- [`cmd/seed-canary-regression/`](cmd/seed-canary-regression) — the demo
  dataset. Run before the session.
- [`internal/scenario/`](internal/scenario) — the scenario itself, with the
  tuning arithmetic documented and tested.
- [`honeycomb-setup/`](honeycomb-setup) — deploy marker and weekly-review board,
  as shell scripts and as Terraform. Any attendee can recreate them.

The Collector from
[`../session-1-fundamentals/collector`](../session-1-fundamentals/collector) is
reused as-is.

## The scenario

A canary deploy that is **~40% slower, but only for enterprise users on the
billing endpoint**. It is the worked example from Chapter 2's Practice 5:

> "You can see that the new build is 40% slower for users on the enterprise plan
> hitting the billing endpoint. That signal is available at 1%. By the time it's
> moving aggregate metrics, it's already affecting a lot of people."

Roughly 50 of 10,000 requests are affected — about 0.5% of traffic. Overall P50
does not move. That gap is the entire point: a dashboard would not have caught
this, and the core analysis loop finds it in under a minute because every event
carries the dimensions needed to explain it.

The regression requires **all three** of canary build, enterprise plan, and the
billing route. Any two is not enough, which is why single-dimension views miss
it.

## Pre-session checklist

1. Start the Collector:

   ```bash
   cd ../session-1-fundamentals/collector
   HONEYCOMB_API_KEY=<send-events key> docker compose up -d
   ```

2. Seed the dataset and note the deploy timestamp it prints:

   ```bash
   go run ./cmd/seed-canary-regression
   ```

3. Create the deploy marker at that timestamp — see
   [`honeycomb-setup/`](honeycomb-setup). Without it the before/after boundary
   in the demo has nothing to anchor to.

4. Confirm in Honeycomb: 10,000 root spans across the window, both
   `service.version` values present, and a visible second latency band on
   `/api/billing` after the marker.

## Live demo run order

1. **P95 by build** — barely moves. Say so out loud; that's the setup.
2. **Heatmap of `/api/billing` duration** — a second band appears after the
   marker.
3. **BubbleUp the slow band** — `service.version=1.5.0`, `user.type=enterprise`,
   `http.route=/api/billing` rank top.
4. **Drill into one trace** — `render_invoice` is where the time went.
5. **Confirm against the deploy marker** — cause established, not guessed.

## Async lab

1. Map your own delivery system against Chapter 2's six practices (0–5): where
   are you on the ladder, and which practice is the next cheapest win?
2. Recreate the marker and board in your own environment using
   [`honeycomb-setup/`](honeycomb-setup) — either path.
3. Run the core analysis loop yourself on the seeded data, without looking at
   the run order above. Time yourself; the target is under ten minutes from
   "something's off" to "I know what and why."
4. Re-seed with different `SEED_*` values and see how small a regression you can
   still find. This is the "detectable at 1%" claim, tested rather than taken on
   faith.

## Tuning

`SEED_COUNT` (10000), `SEED_WINDOW` (4h), `SEED_DEPLOY_AGO` (90m). The scenario
only works inside a narrow band — enough slow requests to isolate, few enough
that aggregates stay quiet — so read the arithmetic in
[`internal/scenario/scenario.go`](internal/scenario/scenario.go) before changing
these. The tests enforce both bounds and will tell you if a change breaks the
demo.
