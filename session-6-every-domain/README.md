# Session 6: Observability in Every Domain

Code accompanying Masterclass 6. See
[`../masterclass-curriculum-45min.md`](../masterclass-curriculum-45min.md) for
the session outline.

**Book mapping:** Chapter 18 (CI/CD Pipelines), Chapter 19 (Mobile and
Frontend), Chapter 20 (Performance Engineering), Chapter 21 (Large Language
Models), Chapter 22 (Fin's Case Study).

Three live demos, matching the curriculum's own demo annotations — CI/CD,
performance engineering, and LLM observability. Frontend/mobile is
slides-only; there's no repo content for it, same as MC4 illustrated some SLI
formulas without separately seeding them.

## The CI/CD demo also carries a real trace of this repo's own pipeline

Alongside `cmd/seed-cicd-pipeline`'s synthetic dataset, this repo's own
`.circleci/config.yml` runs `verify`, `vulncheck`, and
`collector_config_validate` through
[`honeycombio/buildevents-orb`](https://github.com/honeycombio/buildevents-orb)
— real spans from a real run of the pipeline you're looking at, not a
stand-in. It's deliberately smaller scale (one pipeline run at a time, not
500) and it isn't OTel: `buildevents` predates OTel semconv entirely and
ships its own field names (`job_name`, `build_num`, `branch`, ...) straight
to Honeycomb's classic Events API, not `cicd.pipeline.*`/`vcs.change.id` via
OTLP. `cicdscenario`'s seeded dataset is what carries that semconv story for
querying live — this is texture on top of it: proof that CI/CD tracing is a
same-day addition to a pipeline that already exists, using a tool Honeycomb
shipped years before OTel had CI/CD conventions at all.

`terraform_validate` and `shellcheck` are deliberately left unwrapped —
both run on Alpine images confirmed (empirically — `docker run --entrypoint
which ... bash` and `curl`, both exit 1 on both images) to lack the bash
and curl the orb's commands need.

**Needs, none of which are committed here, with different consequences if
missing:**

- Third-party orbs enabled in this org's CircleCI Security settings. This
  one's a hard blocker for the *whole pipeline* — if it's off, CircleCI
  refuses to process `.circleci/config.yml` at all, so every job fails to
  even start, not just the ones below. Flip this before merging anything
  that touches the orb.
- `BUILDEVENT_APIKEY` (a Honeycomb send-events key) and
  `BUILDEVENT_CIRCLE_API_TOKEN` (a CircleCI personal API token with read
  access to this project). On the upstream `honeycombio` org these are
  meant to come from the `Honeycomb Secrets for Public Repos` CircleCI
  context already attached to the relevant jobs in the workflow — the same
  org-wide context other public honeycombio repos use for release-time
  secrets, though it hasn't been confirmed to already carry these two
  exact names. **If you've forked this repo, you won't have access to that
  context** — either add these two as your own project-level CircleCI env
  vars (drop the `context:` lines, or point them at a context you do
  control), or just leave them unset. Either is a soft failure: missing
  `BUILDEVENT_APIKEY` means `buildevents` just writes events to stdout
  instead of Honeycomb — `otel_setup`, `verify`, `vulncheck`, and
  `collector_config_validate` all still pass, there's simply no
  `cicd-pipeline-live` data to look at; missing
  `BUILDEVENT_CIRCLE_API_TOKEN` fails only `otel_watch` (its polling call
  errors) — nothing requires `otel_watch`, so this doesn't block
  merge-gating checks either.

## The LLM demo runs on real Claude Code telemetry, read this first

Unlike every other demo in this repo, the LLM demo's primary data source
isn't a seeder — it's Claude Code's own OpenTelemetry export, pointed at
session-1's shared Collector. That's real cost (`cost_usd`), real tokens
(`input_tokens`/`output_tokens`), a real model name, and a real multi-step
span hierarchy (`claude_code.interaction` → `claude_code.llm_request` /
`claude_code.tool` → `claude_code.tool.execution`) — confirmed empirically
before this session was built, not assumed from docs. `cmd/seed-llm-eval-pipeline`
supplies only the one thing real telemetry has no signal for at all: a
continuous quality score. See [`internal/llmscenario`](internal/llmscenario)'s
package comment for why that split, not a fully synthetic dataset.

**Enable it** (any terminal, pointed at the same Collector every other
session uses):

```bash
export CLAUDE_CODE_ENABLE_TELEMETRY=1
export OTEL_LOGS_EXPORTER=otlp
export OTEL_TRACES_EXPORTER=otlp
export CLAUDE_CODE_ENHANCED_TELEMETRY_BETA=1   # traces are beta; see below
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4317
export OTEL_EXPORTER_OTLP_PROTOCOL=grpc
```

Then just use Claude Code normally — the live demo *is* whatever real usage
happens on stage, no seeding required for this part.

**Privacy, read before going live.** Claude Code's telemetry carries four
real identifiers on every span and log record: `user.account_id`,
`organization.id`, and `user.account_uuid` are masked to `****` by
session-1's Collector (see its config's `redaction` processor, not
deletion); `user.email` passes through deliberately. If that's not the right
call for your team's Honeycomb instance, change the `redaction` processor's
`blocked_key_patterns` before going live, not after.

**Traces are beta** (`CLAUDE_CODE_ENHANCED_TELEMETRY_BETA=1`), and exact
per-span attributes beyond what this repo already verified aren't fully
documented. What's confirmed, empirically, as of this session being built:
`claude_code.interaction` (root, one per turn), `claude_code.llm_request`,
`claude_code.tool` (`tool_name`, `duration_ms`), `claude_code.tool.execution`
(`success`), each carrying `model`, `input_tokens`, `output_tokens`,
`cost_usd`, `duration_ms` on the associated `claude_code.api_request` log
event. If a future Claude Code release changes this shape, re-verify before
the next time you give this session — the same discipline MC5 applies to its
own experimental Collector processor.

## Layout

- [`cmd/seed-cicd-pipeline/`](cmd/seed-cicd-pipeline),
  [`cmd/seed-perf-service/`](cmd/seed-perf-service),
  [`cmd/seed-llm-eval-pipeline/`](cmd/seed-llm-eval-pipeline) — the three
  seeded datasets.
- [`internal/cicdscenario/`](internal/cicdscenario),
  [`internal/perfscenario/`](internal/perfscenario),
  [`internal/llmscenario/`](internal/llmscenario) — the three scenarios,
  with the tuning arithmetic documented and tested.
- [`honeycomb-setup/`](honeycomb-setup) — saved queries, a trigger, and a
  marker across all three seeded datasets, as Terraform.

The Collector from
[`../session-1-fundamentals/collector`](../session-1-fundamentals/collector)
is shared with every other session. Session 6 is why it now redacts three
Claude Code identifiers and carries a `logs` pipeline — see that directory's
`otel-collector-config.yaml`.

## The scenarios

**CI/CD** (`cicd-pipeline` dataset): one pipeline run many times over two
weeks. Two independent, deliberately unrelated failure modes: the build
task's duration steps from ~12m to ~22m in the trailing day (a real
regression, not a gradual drift), and one test case
(`TestPaymentRetryIsIdempotent`) fails ~15% of the time regardless of the
build regression. Real OTel CI/CD semantic conventions throughout
(`cicd.pipeline.*`, `vcs.change.id`), not invented attribute names.

**Performance** (`perf-service` dataset): five parameterized
`db.query.text` templates — three fast, one bimodal (a missing index on one
status value), one long-tail — plus a Graviton (amd64 → arm64) migration
marker halfway through the window with no latency regression across it, and
a `user.type` correlation (enterprise requests run slower). See
[`internal/perfscenario`](internal/perfscenario)'s package comment for why a
bimodal heatmap and a long-tail heatmap need to actually look different for
Chapter 20's claim to be true of this data.

**LLM eval** (`llm-eval-pipeline` dataset): see above — this is the small
supplement, not the primary demo.

## Pre-session checklist

1. Start the Collector (Honeycomb's distro image, redaction + tail-sampling
   already configured — see
   [`../session-1-fundamentals/collector`](../session-1-fundamentals/collector)):

   ```bash
   cd ../session-1-fundamentals/collector
   HONEYCOMB_API_KEY=<send-events key> docker compose up -d
   ```

2. Seed all three datasets:

   ```bash
   go run ./cmd/seed-cicd-pipeline
   go run ./cmd/seed-perf-service    # note the printed migration timestamp
   go run ./cmd/seed-llm-eval-pipeline
   ```

3. Apply the Terraform — see [`honeycomb-setup/`](honeycomb-setup).

4. Confirm in Honeycomb: the CI/CD build-P95 trigger is alerting (or close
   to it), the perf dataset's amd64/arm64 P50-P95 pairs read as close to
   each other, and the LLM SLI query reads meaningfully below 1.0.

5. Set the Claude Code telemetry env vars above in whatever terminal you'll
   actually be typing in during the LLM section.

## Live demo run order

**CI/CD pipelines** (10 min): show the build-P95 trigger firing (or about
to), click into `cicd_flaky_tests` to find the one flaky test case by name.
If the prerequisites above are set up, also pull up `cicd-pipeline-live` —
a real trace of this repo's own most recent CI run — as the "and yes, it's
this easy to add to a pipeline you already have" beat.

**Performance engineering** (10 min): `perf_slow_queries` ranks the bimodal
and long-tail queries above the three fast ones — then `perf_query_heatmap`
shows why a single P99 number can't tell them apart. `perf_duration_by_user_type`
for the correlation step. Close on `perf_duration_amd64` vs
`perf_duration_arm64` — the Graviton story, told by production data.

**LLM observability** (12 min): use Claude Code live with telemetry
enabled — a real multi-step trace lands in Honeycomb in real time. Show
`cost_usd` and token counts on the `claude_code.api_request` events. Then
`llm_sli` for the quality angle real telemetry can't provide on its own.

## Async lab

Pick whichever domain is closest to your work and instrument one thing: a
single CI job step, a slow query, or one LLM call with token and cost
attributes (or just point your own Claude Code sessions at your own
Honeycomb team, using the env vars above). Get it into the workshop
environment and run one query you couldn't run before.

## Tuning

`SEED_COUNT` (500 runs / 24000 requests / 5000 conversations, respectively).
Everything else — route/task/quality proportions, the regression and
migration timing — is in each `internal/*scenario/scenario.go`'s
`DefaultConfig`; the tests enforce the bounds that keep each demo working.
