# The Chapter 2 Ladder

A self-assessment for Chapter 2's six practices. Run it against your own
delivery system, not a hypothetical one.

> "Observability is Practice 0, not Practice 7, because it's the keystone:
> every practice above it is more valuable with it, and unsafe without it."

Two things to hold onto while you fill this in:

- **It's a maturity assessment, not a checklist.** Most teams sit around
  Practice 2 and believe they're at Practice 5. That gap is normal, and
  finding it is the point.
- **Lower beats higher.** The practices build on each other in order. A
  canary (Practice 5) is only trustworthy if the signal that judges it is
  good (Practice 0). Don't chase a practice above the first one you answered
  "no" to — fix that rung first.

---

## Practice 0 — Rich Data and Precision Tooling

The keystone. Feature flags are only useful if you can slice by flag.
Canaries only help if you can compare canary against baseline. Automated
rollbacks are only safe if the signal triggering them is trustworthy. Build
the practices above this on aggregated metrics instead, and you'll conclude
the practices don't work — when what actually failed was the foundation.

```
Can you filter any graph by an arbitrary high-cardinality field (a
specific user, build, or customer) without shipping new code or building
a new dashboard first?                                      yes / no

Last time you had to guess instead of query:
```

---

## Practice 1 — A Feedback Loop Between Developers and Production

> "Look at it before you forget what you were trying to do."

Development isn't done until it's working in production. This practice is
whether that loop exists at all, for you personally, not just for whoever's
on call.

```
After you deploy, do you look at what happened yourself — or does
"somebody" look, only if something breaks?

Typical gap between your deploy and your first look at it in production:
```

---

## Practice 2 — Test Before You Deploy

The bar isn't "no mistakes." It's: **any mistake you find in production is a
new one** — a category your tests hadn't seen, not a repeat of one they
should have caught.

```
Of your last 5 production incidents, how many were a genuinely new kind
of mistake versus a repeat your test suite should have caught?

New: ______   Repeat: ______
```

---

## Practice 3 — Instrument and Validate in Production

Everyone tests in production — a perfect staging replica doesn't exist,
and every deploy meets a unique intersection of real traffic. The practice
is doing it *on purpose*, with a plan, instead of finding out by accident
when it breaks.

```
Do you have a deliberate plan for validating each deploy in production —
or do you only learn you tested in prod when something goes wrong?

On purpose / by accident (circle one)
```

---

## Practice 4 — Feature Flags Decouple Deploys from Releases

Deploys are engineering; releases are product. If those two words mean the
same thing on your team, you don't have this practice yet.

```
Can you ship code to production without exposing it to any user?  yes / no

If yes, what fraction of changes actually go out behind a flag rather
than live at deploy time?                                    ______%
```

---

## Practice 5 — Progressive Delivery: Canaries and Automated Rollbacks

The top rung, and the one that's unsafe to buy first. A canary needs an
automatic comparison against baseline; a rollback needs to fire on a signal
you trust without a human paging in first.

```
Is a canary's behaviour automatically compared against a baseline, or
does a human have to notice the difference?             automatic / manual

Can a bad canary roll back without a human in the loop?        yes / no
```

---

## Find Your Cheapest Next Win

Go back through Practices 0–5 in order. The first one where you answered
"no," "manual," "by accident," or "repeat" more than you'd like — that's
your rung. Not the one that sounds most impressive to fix, the first one
you hit.

```
Lowest practice with a real gap:                              Practice ___
Why that one, and not a higher rung that feels more urgent:
```

---

## Try It Yourself

Two ways to make this concrete instead of theoretical, both in this
directory:

- **Recreate the deploy marker and the weekly-review board** in your own
  environment — [`honeycomb-setup/`](honeycomb-setup) ships both a shell
  script and a Terraform config. This is Practice 1's technical backbone:
  a marker turns "latency looks off" into "latency looks off since 13:12."
- **Run the core analysis loop cold** against the seeded canary regression
  (`go run ./cmd/seed-canary-regression`), without looking at the live
  demo's run order first. Time yourself — under ten minutes is the target,
  and it's a direct test of how much Practice 0 you actually have.

Stretch: re-seed with a smaller canary share (`SEED_*` env vars, documented
in the main [`README`](README.md)) and find how small a regression you can
still detect. That's the book's "available at 1%" claim, tested rather than
taken on faith.
