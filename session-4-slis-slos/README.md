# Session 4: SLIs and SLOs for the Modern Era

Code accompanying Masterclass 4. See
[`../masterclass-curriculum-45min.md`](../masterclass-curriculum-45min.md) for
the session outline.

**Book mapping:** Chapter 11 (Using Service Level Objectives for Reliability),
Chapter 12 (Acting On and Debugging SLO-Based Alerts).

## The free-tier caveat, read this first

Honeycomb Free does not include the native `honeycombio_slo` /
`honeycombio_burn_alert` objects — SLOs are a paid-plan feature. Everything in
this directory is the **workshop path**: the same underlying math (a boolean
SLI, `AVG()` over a scoped, filtered query, and an alert on a trailing window),
built out of a `honeycombio_derived_column`, saved queries, and a plain
`honeycombio_trigger` instead. Any attendee on Free can run every piece of this.

If you're delivering the live session from a team with SLOs enabled, the native
UI is worth showing too — but that's a live demo you drive by hand in the
Honeycomb UI, not something this repo scripts, since the point of the async lab
is that attendees can reproduce it on the plan they actually have.

## Layout

- [`cmd/seed-sli-service/`](cmd/seed-sli-service) — the demo dataset. Run
  before the session, and again shortly before you go live (see below —
  the incident is deliberately still "ongoing" relative to seed time).
- [`internal/scenario/`](internal/scenario) — the scenario itself, with the
  tuning arithmetic documented and tested.
- [`honeycomb-setup/`](honeycomb-setup) — the derived column, saved queries,
  trigger, and incident marker, as Terraform. There is no shell-script
  alternative here (unlike session 2) — a derived column plus a trigger with a
  recipient block is enough surface area that hand-rolled `curl` calls would be
  more code to get subtly wrong than the Terraform itself.

The Collector from
[`../session-1-fundamentals/collector`](../session-1-fundamentals/collector)
is reused as-is.

## The scenario

A service that is mostly healthy. A minority of traffic still runs an older
build that carries a latent defect: for a recent, ongoing window, that build's
requests from **enterprise customers** fail at an elevated rate. Everywhere
else — other plans, other times, `/healthz` — the error rate is the ordinary
background rate.

That gives the session's two live demos real data to work with:

- **Four SLIs, four service types**: the dataset backs the HTTP-API formula
  live (`count(status_code < 500) / count(*) where route != '/healthz'`); the
  other three formulas on that slide are illustrated, not separately seeded.
- **From burn to fix**: the incident is what the trailing-window trigger finds,
  and `service.version` + `user.type` are what a BubbleUp box (or the
  breakdown query Terraform saves) surfaces as overrepresented.

`/healthz` traffic is deliberately high-volume and always succeeds, so the
`route != '/healthz'` clause in the SLI formula has something real to exclude
— leave it out and the incident gets diluted into looking smaller than it is.
See [`internal/scenario/scenario.go`](internal/scenario/scenario.go)'s package
comment for the exact proportions and why they're tuned the way they are.

## Pre-session checklist

1. Start the Collector:

   ```bash
   cd ../session-1-fundamentals/collector
   HONEYCOMB_API_KEY=<send-events key> docker compose up -d
   ```

2. Seed the dataset and note the incident-start timestamp it prints:

   ```bash
   go run ./cmd/seed-sli-service
   ```

   Do this shortly before you go live, not the night before. The incident
   window is anchored to "now" at seed time and is still ongoing when seeding
   finishes — reseed close to the session so the trailing-1-hour SLI query and
   the trigger both have something current to find, rather than something that
   reads as history.

3. Apply the Terraform (derived column, queries, trigger, marker) — see
   [`honeycomb-setup/`](honeycomb-setup).

4. Confirm in Honeycomb: the `baseline_sli` query reads comfortably healthy
   (≳0.95), the `burn_trigger_sli` query reads well below `burn_threshold`
   (default 0.90), and the trigger shows as alerting.

## Live demo run order

**Four SLIs, four service types** (the derived column, `AVG()` demo):

1. Show the `honeycombio_derived_column.sli_good_request` expression in
   Terraform, or create it live in the UI: `IF(LT($http.response.status_code,
   500), 1, 0)`.
2. Run `AVG()` on it, filtered to `route != '/healthz'` — that number is the
   SLI.
3. Toggle the filter off and show the ratio move; that's the exclusion clause
   earning its place in the formula, not a rule to take on faith.

**From burn to fix** (the trigger demo):

1. Open the trigger; it's already alerting (or fires within a few minutes of
   the window rolling forward — its frequency is 5 minutes).
2. Click into the `burn_breakdown_sli` query — sorted ascending, so the worst
   `service.version` + `user.type` combination is the first row.
3. Draw a BubbleUp box around the errors in that same query to confirm.
4. Land on: burn detected, cohort identified, without leaving the page.

## Async lab

1. Write an SLI for a service you own, and validate it against real historical
   data — not this seeded dataset.
2. Sanity-check it with BubbleUp before committing to it.
3. Draft an error-budget policy: what happens at 50% burned, at 100%, who gets
   paged, and what action follows. The Terraform here enforces a threshold; it
   cannot enforce a policy nobody wrote down.
4. If your plan has SLOs, rebuild this same SLI as a native `honeycombio_slo`
   and compare: same derived column, same math, a better UI on top.

## Tuning

`SEED_COUNT` (15000), `SEED_WINDOW` (6h), `SEED_INCIDENT_AGO` (60m). The
incident population is the product of four independent filters (recent, not
`/healthz`, enterprise, buggy build) that compound fast — read the arithmetic
in [`internal/scenario/scenario.go`](internal/scenario/scenario.go) before
changing the shares. The tests enforce the bounds that keep the demo working
and will tell you if a change breaks it.
