# Observability Engineering Masterclass

Code and Honeycomb examples accompanying the six-session masterclass based on
*Observability Engineering* (2nd Edition, O'Reilly). See
[`masterclass-curriculum-45min.md`](masterclass-curriculum-45min.md) for the
full curriculum.

Each session gets its own directory with the demo code and lab materials for
that session. So far:

- [`session-1-fundamentals/`](session-1-fundamentals) — Wide Events &
  Instrumentation with OTel

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
- A Honeycomb API key (`HONEYCOMB_API_KEY`) with send-events permission
