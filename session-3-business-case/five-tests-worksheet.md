# The Five Tests

A worksheet for Chapter 28's diagnostic: *Recognizing Legacy Masquerading as
Modern Observability*. Run it before you argue for investment, and run it again
afterward.

> "Before organizational change can happen, you need to see clearly where you
> actually stand. Not where the vendor says you stand. Not where your dashboards
> say you stand. Where the evidence says you stand." — Chapter 28

Two things to hold onto while you fill this in:

- **It is not a scorecard.** The book is explicit. The output is evidence, and
  the point of evidence is that it "cannot be dismissed as opinion."
- **Rerun it.** Chapter 28's roadmap uses these tests twice — once to establish
  where you are, and again to demonstrate wins. A single run tells you less than
  half of what two runs tell you.

Three of the five are conversations. Two you can answer from data, and this repo
gives you the queries.

---

## 1. The Ownership Test

**Ask any engineer:** "After you merge code, how do you know whether it is
working in production?"

| Legacy | Modern |
|---|---|
| Code goes into a black hole. You do not know when it deployed or whether it works. If it was bad, someone might get paged hours or days later — or a customer finds it months from now. | Code goes live within minutes. You get a notification with a link to your build being canaried. You watch it. If it regresses you see it immediately. |

The real question underneath: **how much energy, prior knowledge, and creativity
do you demand of every engineer just to understand the code they shipped?**

> If the feedback loop does not exist, it does not matter how much software you
> build or buy. You are pouring water into a leaky bucket.

**Your evidence:**

```
Who you asked:
What they said:
Where you actually are:
```

---

## 2. The Two-or-Three People Test

**Ask:** when an engineer needs to answer a question about production, do they
pull up their tools, or do they turn and ask one of the two or three people who
know how to use the tools?

| Legacy | Modern |
|---|---|
| Only a handful of senior engineers can investigate. Everyone else is helpless. When those specialists leave, the knowledge leaves with them. | **60–80% of engineers can independently investigate issues.** Knowledge is encoded in the system rather than in people's heads. |

In a large enterprise this is not a skills gap, it is a structural
vulnerability: those two or three people are single points of failure across
hundreds of engineers, they get pulled into every incident, they become
bottlenecks, and then they burn out.

> If your observability requires a firefighter, you do not have observability.
> You have a dependency on humans who will eventually leave.

### This one you can measure

Honeycomb records who runs queries in the **Activity Log** environment, which
exists on every team — yours included. Its `query_results` dataset has one event
per query run, retained for a year.

**How to get at it.** Two verified paths, and one that is not:

| Path | Who can use it |
|---|---|
| **Honeycomb UI** — Environments → Activity Log → `query_results` | All team members |
| **Honeycomb MCP server**, from an agent | Team Owners only |
| Query Data REST API | Not documented for this environment; do not assume it works |

Run it in the UI unless you are a Team Owner with MCP wired up. The queries
below are written as UI query-builder settings for that reason.

**Query A — how many people actually query, and how?**

- Environment: `Activity Log` (slug `$activity-log$`)
- Dataset: `query_results`
- Time range: `30 days`
- Calculate: `COUNT` and `COUNT_DISTINCT(user.email)`
- Group by: `source`
- Where: `user.email exists`

The `user.email exists` filter matters: queries run with an API key have no user
attached, and counting them would flatter you. `source` separates how people
investigate — `ui` is the query builder, `mcp` is an engineer asking through an
AI agent, `investigation` and `canvas` are the guided flows, `api` is
automation.

**Query B — the concentration. This is the actual test.**

- Same environment, dataset, time range, and filter
- Calculate: `COUNT`
- Group by: `user.email`
- Order by: `COUNT` descending
- Limit: `50`

Then compute:

```
Engineers in the organisation:                      ______
Distinct people who ran a query (30d):              ______
  ... as a share of engineers:                      ______%   <- the 60-80% line

Queries run by the top 3 people:                    ______
Total queries:                                      ______
  ... top 3 as a share of all queries:              ______%   <- the bottleneck
```

If the second number is small or the last number is large, you have found your
two-or-three people. That is not a criticism of them; see "Honor the Heroes Who
Held It Together" in Chapter 28.

**One thing worth noticing in the `source` breakdown:** a healthy share of
`mcp`-sourced queries is a signal in its own right. An engineer who cannot yet
write the query can still ask the question, which is one of the few levers that
actually moves this number rather than just measuring it.

---

## 3. The Mystery Test

**Ask engineers directly:** "What's something weird about production that we
just accept?"

| Legacy | Modern |
|---|---|
| Shallow mysteries persist for months. "We have a load spike around midnight and don't know where it comes from." "That job just fails sometimes." "Latency is higher on Tuesdays." People learned to stop investigating. | Mysteries become specific and investigable: "we have a load spike" becomes "the batch analytics job triggers 40,000 database queries because the query plan changed." |

> Mysteries that persist are not mysteries. They are the shape of your blindness.

**Ways to surface this without waiting for a meeting**, all from the book:

- Search chat history for "just does that sometimes" and "we never figured out".
- Count how many recent postmortems list root cause as unknown or
  indeterminate.
- Check runbooks for procedures that amount to "restart and hope".

**Your evidence:**

```
Mysteries named:
Postmortems with unknown root cause (last 10):     ______ / 10
Runbook steps that are "restart and hope":         ______
```

---

## 4. The Arbitrary Question Test

**Give an engineer a question nobody preconfigured for.** The book's example:

> "Show me all failed checkout attempts from mobile users in California using
> version 2.3.1 of the app during lunch hour over the past week."

| Legacy | Modern |
|---|---|
| They check whether those dimensions were indexed. Probably not. They wait hours or days for indexes, resort to manual log analysis, or give up: "we don't track that combination." | **They answer in under 60 seconds.** The data exists, the query runs, the answer appears. |

> Monitoring answers questions you predicted. Observability answers questions
> you did not.

### Run it yourself

`cmd/seed-arbitrary-question` in this directory generates a week of wide
checkout events so you can try exactly this. It prints the filters and how many
events match before you go looking, so you know whether an empty result means
"no such requests" or "you typed it wrong."

**Then do the version that counts:** ask it of *your* production data, with a
question your team has never been asked. Time it.

```
The question you invented:
Could it be answered at all?              yes / no
Time to an answer:                        ______
Dimensions you turned out not to have:
```

---

## 5. The Deployment Confidence Test

**Ask engineering leadership:** "How much fear is there around deployments?"

| Legacy | Modern |
|---|---|
| Deploys happen weekly or less, need war rooms, and frighten people. Read-only Fridays. Junior engineers not allowed to deploy. Change Advisory Boards exist because slowing down is the only safety mechanism anyone trusts. | Deploys happen multiple times daily, automated and routine. Engineers deploy confidently because they can see the impact. Rollbacks are fast. |

> Fear is a signal. They are not afraid of deploying. They are afraid of not
> knowing.

**Your evidence:**

```
Deploys per day (median):                           ______
Read-only Fridays or equivalent?              yes / no
Can a first-week engineer deploy?             yes / no
Does a CAB gate routine changes?              yes / no
```

---

## After the tests

The book is direct about what comes next, and it is not a purchase order:

> Closing those gaps is not about buying more tools or hiring more people. It is
> about changing how your organization thinks about production, who owns what,
> and what "good" actually looks like. That is organizational change.

Expect resistance, and expect it to be rational rather than obstinate — vendors
protecting contracts, teams protecting professional identity, leadership
avoiding the admission that money went to the wrong things. Chapter 28's
"organizational immune response" is worth reading before your first meeting.

The test results are what you bring to that fight.

## Then build the case

Turn the gaps into a cost with `cmd/business-case`, using your own numbers:

```bash
go run ./cmd/business-case \
  -engineers 120 -loaded-cost 260000 \
  -incidents-per-month 14 -mttr-hours 3.5 -responders 3 \
  -escalation-rate 0.45 -escalation-extra 2 \
  -unplanned-share 0.22 -observability-spend 350000
```

Note that the escalation rate you feed it is the Two-or-Three People Test
expressed as money, which is why the two halves of this worksheet belong
together.

It prints its arithmetic and a range across three labelled assumptions, and it
will happily tell you the investment does not pay for itself. Chapter 26 declines
to hand over an ROI formula — "turning this into monetary estimates is harder to
give guidance on" — so this does not pretend to be one.
