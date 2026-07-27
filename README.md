# Observability Engineering Masterclass

Code and Honeycomb examples accompanying the six-session masterclass based on
*Observability Engineering* (2nd Edition, O'Reilly). See
[`masterclass-curriculum-45min.md`](masterclass-curriculum-45min.md) for the
full curriculum.

Each session gets its own directory with the demo code and lab materials for
that session. So far:

- [`session-1-fundamentals/`](session-1-fundamentals) — Wide Events &
  Instrumentation with OTel
- [`session-2-feedback-loops/`](session-2-feedback-loops) — Observability as the
  Connective Tissue of Delivery

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
- A Honeycomb configuration key, if you want to create markers/boards/SLOs —
  a different key from the one above
- Terraform 1.5+ (optional; every Honeycomb object also has a shell-script path)

Run `make verify` to build, vet, test, and format-check every module. The repo
root is a Go workspace over several per-demo modules rather than a module
itself, so use the Makefile rather than `go build ./...` from here.
