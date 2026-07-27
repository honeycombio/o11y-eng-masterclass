# Session 3: The Business Case and the Investment Diagnostic

Code accompanying Masterclass 3. See
[`../masterclass-curriculum-45min.md`](../masterclass-curriculum-45min.md) for
the session outline.

**Book mapping:** Chapter 17 (Telemetry Management), Chapter 26 (The Business
Case for Observability), Chapter 27 (Diagnosing Your Observability Investment),
Chapter 28 (The Organizational Shift).

This session is for leaders, and its centre of gravity is a worksheet rather
than a terminal. The code here exists to make two of the five diagnostic tests
answerable from evidence instead of opinion.

## Layout

- [`five-tests-worksheet.md`](five-tests-worksheet.md) — **start here.** Chapter
  28's five named tests as a runnable diagnostic. This is the async lab and the
  artifact attendees take to their next budget conversation.
- [`cmd/seed-arbitrary-question/`](cmd/seed-arbitrary-question) — a week of
  genuinely wide checkout events, so the Arbitrary Question Test can be run live.
- [`cmd/business-case/`](cmd/business-case) — turns your own numbers into a
  one-page argument, with the arithmetic shown.
- [`internal/wideevent/`](internal/wideevent),
  [`internal/businesscase/`](internal/businesscase) — the generator and the
  model, both tested.

The Collector from
[`../session-1-fundamentals/collector`](../session-1-fundamentals/collector) is
reused as-is.

## The five tests

Chapter 28's diagnostic, and the spine of the session:

| Test | The question | Verdict rests on |
|---|---|---|
| Ownership | "After you merge, how do you know it's working in production?" | A conversation |
| Two-or-Three People | Do engineers open the tools, or ask the two or three who can? | **Your Activity Log data** |
| Mystery | "What's weird about production that we just accept?" | A conversation, plus postmortems and runbooks |
| Arbitrary Question | A question nobody preconfigured for | **A live query, timed** |
| Deployment Confidence | "How much fear is there around deployments?" | A conversation |

Two are measurable, and that is the point: the book insists on evidence that
"cannot be dismissed as opinion."

## Pre-session checklist

1. Start the Collector:

   ```bash
   cd ../session-1-fundamentals/collector
   HONEYCOMB_API_KEY=<send-events key> docker compose up -d
   ```

2. Seed the Arbitrary Question dataset:

   ```bash
   go run ./cmd/seed-arbitrary-question
   ```

   It prints the exact filters and how many events match. **Check that number is
   non-zero before going live** — an empty result on stage is the failure mode,
   and reseeding takes a second.

3. Open the Activity Log in the UI (Environments → Activity Log →
   `query_results`) and confirm the Two-or-Three People queries return data for
   your team. See the worksheet for the query settings.

4. Have `cmd/business-case` ready in a terminal with your own numbers, not the
   placeholders.

This seeds its own `checkout-web` dataset rather than the `sample-service` one
Masterclass 1 and 2 use, so rehearsing all three in a row does not drop a week
of differently-shaped data inside their 4-hour windows.

## Live demo run order

1. **The Arbitrary Question Test.** Read the book's question aloud, then answer
   it. The dataset was not built for it — it carries ~28 attributes and the
   question needs four. Time yourself; the claim is under 60 seconds.
2. **The Two-or-Three People Test**, against real Honeycomb data. Break down
   query volume by `source`, then by `user.email`. The shape of that second
   result is the diagnosis.
3. **The business case**, built from the numbers the tests just produced — the
   escalation rate especially, which is the Two-or-Three People Test expressed
   as money.

## Async lab

1. Run all five tests on your own organisation using the worksheet. Three are
   conversations; have them.
2. Run the Two-or-Three People queries against your own Activity Log. Compute
   distinct queriers as a share of engineers, against the book's 60–80% line.
3. Invent an arbitrary question nobody has asked of your production data, and
   time how long it takes to answer — or to establish that it cannot be.
4. Build the one-page case with `cmd/business-case` using three real numbers you
   can source: MTTR, incidents per month, and share of time on unplanned work.
5. Diarise a rerun. Chapter 28 uses these tests twice, and the second run is
   what turns a plan into evidence.

## A note on the numbers

`cmd/business-case` deliberately is not an ROI calculator. Chapter 26 cites
public figures for the latency–revenue link (Amazon, Walmart, Staples — run
`go run ./cmd/business-case -sources`) and then says outright that "turning this
into monetary estimates is harder to give guidance on."

So the tool computes the cost of the current state from auditable inputs, shows
every formula, reports a range across three labelled assumptions, and will tell
you when the investment does not pay for itself. A case that only works at its
optimistic end is not a case.
