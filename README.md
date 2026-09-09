# Observability Engineering Masterclass

Code and Honeycomb examples accompanying the six-session masterclass based on
*Observability Engineering* (2nd Edition, O'Reilly). See
[`masterclass-curriculum-45min.md`](masterclass-curriculum-45min.md) for the
full curriculum.

Each session gets its own directory with the demo code and lab materials for
that session. So far:

- [`session-1-fundamentals/`](session-1-fundamentals) — Wide Events &
  Instrumentation with OTel
- [`session-2-feedback-loops/`](session-2-feedback-loops) — Observability Connects Code
  to Delivery
- [`session-3-business-case/`](session-3-business-case) — The Business Case and
  the Investment Diagnostic
- [`session-4-slis-slos/`](session-4-slis-slos) — SLIs and SLOs for the Modern
  Era
- [`session-5-cost-sampling/`](session-5-cost-sampling) — What Observability
  Costs, and How to Make It Cost Less
- [`session-6-every-domain/`](session-6-every-domain) — Observability in
  Every Domain

## Delivery model

Each masterclass is a 45-minute live session, run from pre-built and
pre-seeded demos — nothing is built from scratch on stage. The same
materials also work as a longer, self-serve async lab, or as raw material
for a longer instructor-led workshop; those formats aren't time-boxed to 45
minutes. Every session directory calls out which parts are "pre-session
setup," which are "live demo," and which are lab/workshop material.

## Requirements

- Go 1.26+
- Docker (for the local OTel Collector)
- A Honeycomb API key with send-events permission, for the Collector
- A Honeycomb configuration key, if you want to create markers/boards/triggers —
  a different key from the one above
- Terraform 1.5+ (optional for sessions 1-3; every Honeycomb object there also
  has a shell-script path. Sessions 4, 5, and 6 are Terraform-only)
- Claude Code, with telemetry enabled, for session 6's LLM demo — see that
  session's README

## Verifying

```bash
make verify      # gofmt, build, vet, test across every module
make vulncheck   # govulncheck per module
make tidy-check  # fails if go.mod/go.sum are not tidy
```

The repo root is a Go workspace (`go.work`) over several per-demo modules rather
than a module itself, so use the Makefile rather than `go build ./...` here.

CI ([`.circleci/config.yml`](.circleci/config.yml)) runs all of the above plus
`terraform fmt`/`validate`, `shellcheck`, and — since sessions 5 and 6 added
processors to session-1's shared Collector config — `otelcol validate` against
that config. Two things it is specifically there to catch, because both have
already happened once:

- **Dropped spans.** The seeders' delivery tests stand up an in-process OTLP
  receiver and assert every generated span actually arrives. A seeder that
  flushed only at the end silently lost ~95% of its data while exiting zero,
  and no unit test noticed — the traces were built correctly, they just never
  landed. Each delivery test also asserts its own seed volume exceeds the span
  queue, since below that threshold it would pass on broken code.
- **Dependency drift.** A module added after a security bump resolved gRPC back
  down to the vulnerable version, because indirect dependencies track the module
  graph minimum. `vulncheck` and `tidy-check` catch that class.
