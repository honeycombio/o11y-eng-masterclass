# Observability Masterclass Curriculum

## Based on *Observability Engineering* (2nd Edition, O'Reilly)

Six masterclasses for engineering leaders who want to move from theory to practice, each connecting the book to concrete workflows and decision frameworks.

**Format:** Each session is ~45 minutes of live, demo-paced content. I prepare the material, run through it live with working demos, and the recording plus a self-serve lab are available to work through async at your own pace. I don't stop mid-session for participants to catch up; the async labs exist precisely so you can replay and try things yourself without holding up the room. No TAs required, no three-hour drawn-out workshop.

These sessions use Honeycomb as the reference implementation for the demos; the concepts apply regardless of your current tooling, and I flag what's Honeycomb-specific versus universal. Chapter references map to the published 2nd edition.

**Sequence at a glance:**

1. Fundamentals: Wide Events & Instrumentation with OTel
2. Feedback Loops: Observability as the Connective Tissue of Delivery
3. The Business Case and the Investment Diagnostic
4. SLIs and SLOs for the Modern Era
5. What Observability Costs, and How to Make It Cost Less
6. Observability in Every Domain

---

## Masterclass 1: Fundamentals; Wide Events & Instrumentation with OTel

**Premise:** You need the mental model and the means to act on it in the same breath. Logs, metrics, and traces aren't three different things; they're three views over the same wide events. Once that clicks, instrumentation is just the craft of producing good events. Get the model wrong and you'll spend money rebuilding monitoring with a more expensive label.

**Book mapping:** Chapter 1 (What Is Observability?), Chapter 4 (Getting Started with Instrumentation), Chapter 5 (Structured Events Are the Building Blocks of Observability), Chapter 6 (Making Structured Events Arbitrarily Wide), Chapter 7 (Instrumenting Your Code with OpenTelemetry).

**Duration:** ~45 minutes

### What we cover (live, ~45 min)

**The two competing models (8 min).** Chapter 1 frames the whole field as a choice: the three pillars model (separate stores for logs, metrics, and traces) versus the unified storage model. The three pillars are an artifact of vendor history, not a law of physics. When you store data in three separate systems, you can't correlate across them at query time; you're left stitching together timestamps by hand at 3am.

**Why wide structured events win, and a trace is just events with relationships (10 min, live demo).** Start with a log line: `ERROR: request failed for user 12345`. Count it, grep it; that's about all you can do. Now the same moment as a wide structured event with 40+ attributes: `user.id`, `user.type`, `request.path`, `db.query.duration_ms`, `feature_flag.value`, `error.type`. Now you can filter, group, aggregate, correlate, and ask questions you didn't anticipate. Chapter 5's central insight: a trace is a collection of structured events sharing a trace context with parent-child relationships; a metric is a pre-aggregated projection of what events already contain. I'll query one dataset three ways live; as a rate (metric-like), a filtered search (log-like), and a trace waterfall. One data model, three analysis modes. That equivalence is the whole foundation, and the rest of this session is about producing those events well.

**What to capture, in four categories (10 min).** Chapter 6's "arbitrarily wide event" philosophy as a checklist:

1. **Identity:** `user.id`, `customer.id`, `tenant.id`, `session.id`. Your cardinality anchors; everything filters through them.
2. **Request and execution:** `http.method`, `http.route`, `grpc.method`, `db.statement`. Follow OTel semantic conventions; don't invent your own.
3. **Outcome:** `http.status_code`, `error.type`, `duration_ms`. Distinguish 4xx (often the client's fault) from 5xx in your SLIs.
4. **Service and code context:** `service.version`, `deployment.environment`, build info, `feature_flag.value`. This is what lets you correlate incidents with deploys.

You don't need all 40 on day one. Chapter 4's principle: fast and close to right beats perfect. Each week, ask "what couldn't I answer yesterday?" and add those attributes.

**OpenTelemetry in practice (12 min, live demo).** Why OTel now, in 2026: the spec is stable, the SDKs are mature, the Collector is production-grade. You're no longer trading stability for portability. Live, I'll show auto-instrumentation getting you most of the way for free (`otelhttp`, `otelsql`), then adding a custom span for business logic (`tracer.Start()` / `span.SetAttributes()` / `span.End()`), and why you propagate context explicitly rather than relying on thread locals. Plus the three traps that eat the most time: missing `context.Context` passing in Go, missing `with_span` in Python, and noisy library instrumentation (the `noop.NewTracerProvider()` escape hatch). The Collector is where batching, retry, routing, and sampling decisions belong, not your application code; we go deep on sampling in Masterclass 5.

**AI-assisted instrumentation (5 min).** Chapter 7's section on using AI agents to instrument code. I'm a reformed AI sceptic, and this is one place it genuinely helps; with guardrails. AI is good at boilerplate span creation, standard-attribute scaffolding, and proprietary-SDK-to-OTel migration. It's bad at knowing which attributes actually help you debug *your* service, getting sampling right, and following your conventions. AI is an amplifier, not a solution; it amplifies good and bad instrumentation design equally. Treat its output like a PR from a junior engineer: review every attribute name, don't rubber-stamp.

### Async lab (self-serve)

Two parts, do them in order. First, load the sample dataset and reconstruct one trace three ways: a count of error events, a filtered list of slow spans, and a waterfall; feel the difference between "I can count this" and "I can interrogate this." Second, clone the sample Go or Python service, turn on auto-instrumentation, confirm spans arrive, then add one custom span with three attributes from the four categories and verify it by querying for it. Stretch goal: have an AI assistant scaffold instrumentation for a second endpoint, then review and correct its attribute choices against the session's conventions.

---

## Masterclass 2: Feedback Loops; Observability as the Connective Tissue of Delivery

**Premise:** Observability isn't an ops tool; it's the thing that tells you whether every other practice in your delivery system is working. The work of development isn't done until it's working in production.

**Book mapping:** Chapter 2 (How Code Crosses Over), Chapter 8 (Getting Started with Observability Analysis), Chapter 9 (Observability-Driven Development), Chapter 24 (Systems Thinking for Software Delivery).

**Duration:** ~45 minutes

### What we cover (live, ~45 min)

**The core analysis loop, live (15 min, demo).** Chapter 8 distinguishes debugging from known conditions (monitoring) versus debugging from first principles (observability). I'll run the loop live: a deploy ships, latency spikes for one user type, no alert fires. Notice it via SLO burn, drill into slow traces, use automated comparison (BubbleUp) to surface `user.type=enterprise` plus a new code path, confirm against the deploy marker, fix with confidence. Target: "something's wrong" to "I know what and why" in under ten minutes. If your workflow is slower, that's your improvement opportunity.

**Crossing over from dev to production (15 min).** Chapter 2 is the second edition's thesis. Its numbered practices are a maturity ladder: rich data and tooling (Practice 0), a developer-to-production feedback loop (Practice 1), testing before deploy (Practice 2), instrument and validate in production (Practice 3), decouple deploys from releases with feature flags (Practice 4), and progressive delivery with canaries and automated rollbacks (Practice 5). The deploy marker is the connective thread; annotate deploys and every post-deploy query gets a free before/after boundary. This is observability-driven development (Chapter 9) made concrete: before you write the code, think about the span you'll need at 3am.

**Closing the loop (15 min).** Chapter 24 frames delivery as balancing and amplifying feedback loops; observability is what closes them. The weekly observability review (a 30-minute ritual over a shared board: SLO burn, deploy correlation, top error patterns), saved queries so it's low-friction, and query permalinks in your issue tracker so the data that motivated a fix stays attached to the work. If your feedback loops are slow, everything is slow; observability is how you make them fast.

### Async lab (self-serve)

Map your own delivery system against Chapter 2's six practices: where are you on the ladder, and which practice is the next cheapest win? Then set up one deploy marker and one saved board in the workshop environment, and run a before/after query across a simulated deploy. No catch-up needed live; the ladder self-assessment is a worksheet you can do anytime.

---

## Masterclass 3: The Business Case and the Investment Diagnostic

**Premise:** This one is for leaders. Where observability sits in your budget, how to justify the spend, and how to tell whether you're getting what you paid for. Most organisations are paying observability prices for monitoring-level outcomes; this session is how you find out if you're one of them.

**Book mapping:** Chapter 26 (The Business Case for Observability), Chapter 27 (Diagnosing Your Observability Investment), Chapter 28 (The Organizational Shift).

**Duration:** ~45 minutes

### What we cover (live, ~45 min)

**Are you paying o11y prices for monitoring outcomes? (15 min).** Five questions any leader can ask their team: can any engineer debug without escalating; how long from "alert fired" to "root cause"; are dashboards used or just comfort blankets; do SLOs drive prioritisation; can you answer "which customers are affected?" in five minutes? If one person writes 80% of the queries, you have a knowledge bottleneck, not an observability practice. I'll demo Honeycomb's self-telemetry to show what "who's actually querying" looks like in practice.

**The business case (18 min).** Chapter 26, with numbers for your CFO. Cost of a P1 incident isn't just the outage window; it's post-mortem time, churn risk, and opportunity cost. Worked example: SLA breach at US$10k/hour, 45-minute MTTD with monitoring versus 8-minute with trace-driven analysis. Deploy frequency is a leading indicator; teams that can observe production confidently deploy more often, and that's a causal relationship, not a coincidence. The rework-cost anti-pattern: how much engineering time goes into incidents that could have been prevented or resolved faster? Measure observability cost against the engineers it makes effective, not as an isolated line item.

**Governance and the organisational shift (12 min).** Chapters 27 and 28. The difference between a centralised team that owns everything (congratulations, you're the bottleneck) and a platform team that makes good observability the path of least resistance: a small platform team owns the integration, the ontology and conventions (Chapter 17), and the sampling policy; every product team owns its own instrumentation. The anti-pattern to avoid is one person who knows how to write queries; that's a single point of failure, and it's remarkably common.

### Async lab (self-serve)

Build a one-page business case for your own org using three real numbers: current MTTR, deploy frequency, and percentage of incidents that require escalation. Run the five-question diagnostic on your team and note the gaps. No tooling required; this one is a worksheet, and it's the artifact you take into your next budget conversation.

---

## Masterclass 4: SLIs and SLOs for the Modern Era

**Premise:** SLOs have been around a decade and most teams still aren't using them well. Here's what's changed, and why it matters more in an AI-assisted world where change is accelerating.

**Book mapping:** Chapter 11 (Using Service Level Objectives for Reliability), Chapter 12 (Acting On and Debugging SLO-Based Alerts).

**Duration:** ~45 minutes

### What we cover (live, ~45 min)

**Why most SLO implementations fail (8 min).** Chapter 11's diagnosis: threshold alerting creates fatigue and only catches known-unknowns. The failure modes: SLOs set by ops with no product buy-in; SLIs built on infra metrics instead of user experience; error budgets with no policy attached; too many SLOs (if everything's critical, nothing is). User experience is the north star.

**Designing good SLIs (15 min, live demo).** A good SLI measures something users feel, is event-based so you can drill in, and survives code changes. Live, I'll write and validate SLIs for four service types:

- **HTTP API:** `count(status_code < 500) / count(*) where route != '/healthz'`. Exclude health checks; use `<500` not `==200`.
- **Async jobs:** `count(job.status == 'completed') / count(job.status != 'pending')` over a window; mind the time boundary.
- **Database service:** different latency budgets for interactive vs. batch.
- **LLM-backed service:** `count(llm.response.quality_score > 0.7) / count(*)`, where the score comes from an eval pipeline; the 2026 challenge, since the metric needs its own AI subsystem (Chapter 21).

**Targets and burn-rate alerts (12 min).** Targets are a negotiation, not a calculation; start at or just above your 28-day baseline. Chapter 12's alert taxonomy: threshold-crossing, relative burn, and predictive burn alerts, with time framed as a sliding window. The multi-window default (2% burn in 1 hour pages immediately; 5% in 6 hours pages if sustained) combines fast detection with low false positives. Forecasting lets you alert *before* the budget is empty, not after.

**Event-based SLOs and why they win (10 min, demo).** Metric-based SLOs tell you *that* something broke. Event-based SLOs tell you *what*, *for whom*, and *why*. Live: an alert fires, I click into the burn graph, drill into the window, and automated comparison surfaces `service.version=1.4.2` + `user.type=enterprise` as overrepresented; "SLO burning" to "I know what to fix" without leaving the page. The SLO and the debugger are the same system over the same data.

### Async lab (self-serve)

Write an SLI for a service you own and validate it against historical data in the workshop environment; sanity-check it with BubbleUp before committing. Then draft an error-budget policy: what happens at 50% burned, at 100%, who gets paged and what action follows. Tooling can't enforce what the org hasn't decided.

---

## Masterclass 5: What Observability Costs, and How to Make It Cost Less

**Premise:** The sticker shock is real; observability at scale is expensive. But "it's too expensive" is usually a symptom of doing it wrong, not a reason to do less. If you're spending money and getting monitoring outcomes, the fix is better outcomes, not a smaller bill.

**Book mapping:** Chapter 13 (Efficient Data Storage with Retriever), Chapter 14 (Efficient Data Storage with ClickHouse), Chapter 15 (Cheap and Accurate Enough Sampling), Chapter 16 (Telemetry Management with Pipelines), Chapter 27 (Diagnosing Your Observability Investment).

**Duration:** ~45 minutes

### What we cover (live, ~45 min)

**Where the money goes (8 min).** Three drivers: ingest volume (events/sec × event size; every attribute compounds), retention (90 days costs ~3× what 30 does; are you actually querying old data or just hoarding it?), and query compute (auto-refreshing dashboards nobody watches burn budget silently).

**Sampling, the biggest lever (18 min, live demo).** Chapter 15 builds it up: fixed-rate, recording the sample rate, consistent sampling, target-rate, sampling by key, dynamic rates, and finally combined head-and-tail per-key target-rate sampling. The principles: reduce what you never query (annual attribute audit), derive metrics from events instead of running a parallel metric pipeline, and record the sample rate so the query engine corrects for it; sampling is cost control with preserved statistical accuracy, not data loss. Head-based at a fixed rate is the simple default; tail-based per-key (Refinery or a Collector pipeline) keeps 100% of errors and slow traces while sampling the happy path hard. Worked example: US$50k/month, 80% healthy fast traces, sample those aggressively and you're near US$12k/month with no meaningful loss of debugging power. The trap: tail sampling needs stateful infrastructure and complexity; below ~10k traces/sec, head-based is probably right. Don't over-engineer before you need to.

**Pipelines and storage (12 min).** Chapter 16's telemetry pipeline functions; collect, normalise/secure, enrich, reduce, route. Reduce and route are where cost control lives: drop and transform before the backend, route signals to different destinations with different retention. Dataset design tradeoffs (monolithic for easy cross-service queries vs. per-service for retention control). And the self-hosted question (Chapters 13–14): ClickHouse gives you the concepts to evaluate it honestly, but the answer is usually "not until operating it costs less than the licensing delta"; do the full TCO including on-call and the product features you won't build.

**Organisational discipline (7 min).** Short retention (7 days) for non-prod datasets; quarterly board audits (delete what nobody's opened in 90 days); a five-minute monthly bill review asking "are we getting value from this?" Cost should be a forcing function for adoption (Chapter 27).

### Async lab (self-serve)

Pull your own usage data and rank your top three cost drivers; which datasets grow fastest, which services contribute most volume. Then model one sampling change (pick a head-based rate or a tail-based keep-all-errors policy) and estimate the bill impact. Bring the number to your next budget conversation.

---

## Masterclass 6: Observability in Every Domain

**Premise:** Observability isn't just for backend microservices. Wide events, high cardinality, and structured exploration apply everywhere software runs. The book devotes all of Part V to this; here are the highlights, live.

**Book mapping:** Chapter 18 (CI/CD Pipelines), Chapter 19 (Mobile and Frontend), Chapter 20 (Performance Engineering), Chapter 21 (Large Language Models), Chapter 22 (Fin's Case Study).

**Duration:** ~45 minutes

### What we cover (live, ~45 min)

**CI/CD pipelines (10 min, demo).** Chapter 18's claim: build observability has the best ROI of all observability applications. Your pipeline is a distributed system; steps are spans, runs are traces, failed steps are errors. Instrument a span per job step with `git.sha`, `pr.number`, `test.suite.name`; one span per test case to query flakiness and duration. Define SLIs for build reliability the way you do for services. Live: a build-time regression from 12 to 22 minutes caught on day one by a P95-duration trigger instead of going unnoticed for a week.

**Frontend and mobile (8 min).** Chapter 19; you don't control the environment, the network is unreliable, and the user is a real human whose session you reconstruct. The ring buffer pattern: buffer client-side, flush on error or session end, rather than sending everything. Session correlation: propagate `session.id` from frontend through every backend service and pull up the whole user journey as one trace. Real example: a mobile browser version where requests succeeded but the JavaScript rendered checkout wrong; backend metrics said fine, users said otherwise, event-based observability found it.

**Performance engineering (10 min, demo).** Chapter 20; production traffic is your most accurate benchmark, not synthetic load. The workflow: find expensive operations (`P99(duration_ms) GROUP BY db.statement`), characterise the distribution (a bimodal heatmap is a different problem than a long tail), correlate with attributes (`AVG(duration_ms) GROUP BY user.type`), and track optimisation impact across deploy markers. Worked example: our Graviton/ARM64 migration, validated on production data; roughly 30% fewer instances, 40% lower EC2 cost, no latency regression confirmed by P50/P99 comparison. We trusted the production data, not the benchmark.

**LLM observability (12 min, demo).** Chapter 21 pairs two ideas that must work together: evaluations for reliability, and telemetry designed for LLM apps. The new challenges: non-determinism (success/failure can't capture quality, so you need evals), latency variability (P50 fine, P99 catastrophic, and users feel P99), cost per request (`gen_ai.usage.input_tokens` / `output_tokens` plus a cost attribute, non-negotiable), and multi-step pipelines (retrieval, assembly, LLM call, parsing; each its own span). Use the OTel GenAI semantic conventions so queries stay portable. The loop closes when production telemetry feeds your evals and your next iteration; the AI-era version of observability-driven development.

**Pulling it together with Fin (5 min).** Chapter 22, the capstone: how Intercom built and scaled Fin, their AI agent, on fast feedback loops and observability. The throughline for leaders; resolution rate as the north star, the birth of "time to first token" as a user-facing metric (P99 latency and quality both first-class SLIs), cost per resolution instrumented as a real business metric, and empathy tying the technical work back to users.

### Async lab (self-serve)

Pick whichever domain is closest to your work and instrument one thing: a single CI job step, a frontend session, a slow query, or one LLM call with token and cost attributes. Get it into the workshop environment and run one query you couldn't run before. The point is to prove the same primitives transfer to your domain.

---

## Closing Note

When CI/CD, frontend, backend, and AI systems all emit OTel-compatible telemetry into unified storage, you get a single view of your entire delivery system. A slow build shows up in deploy frequency. A frontend error correlates with a backend trace. An LLM quality regression shows up as an SLO burn before a customer files a ticket. That's not monitoring; that's observability as the central nervous system of how you build and operate software.

**Chapter cross-reference:** MC1 → Ch 1, 4, 5, 6, 7; MC2 → Ch 2, 8, 9, 24; MC3 → Ch 26, 27, 28; MC4 → Ch 11, 12; MC5 → Ch 13, 14, 15, 16, 27; MC6 → Ch 18, 19, 20, 21, 22.
