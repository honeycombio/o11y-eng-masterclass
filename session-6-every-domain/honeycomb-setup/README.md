# Honeycomb setup for Masterclass 6

Terraform for three datasets, three demos: CI/CD, performance engineering,
and LLM observability (the eval-quality supplement only — the LLM demo's
main data source is real Claude Code telemetry, not this repo's Terraform;
see [`../README.md`](../README.md)).

## API keys

Two different keys are involved, same split as every other session:

- The **Collector** uses a key with *send events* permission. That's the one
  in `HONEYCOMB_API_KEY` when you start the collector in
  [`../../session-1-fundamentals/collector`](../../session-1-fundamentals/collector).
- **This Terraform** needs a *configuration* key, with permission to manage
  derived columns, queries, triggers, and markers. A send-events key gets a
  401 here.

Honeycomb EU: export `HONEYCOMB_API_ENDPOINT=https://api.eu1.honeycomb.io`.

## Quickstart

```bash
cd ../..                                       # session-6-every-domain
go run ./cmd/seed-cicd-pipeline
go run ./cmd/seed-perf-service                 # prints the migration timestamp
go run ./cmd/seed-llm-eval-pipeline

cd honeycomb-setup/terraform
cp terraform.tfvars.example terraform.tfvars
$EDITOR terraform.tfvars                       # set perf_migration_start from
                                                # the perf seeder's output, and
                                                # cicd_trigger_recipient_target
                                                # to a real address

export HONEYCOMB_API_KEY=<config key>
terraform init
terraform plan
terraform apply
```

Seed before applying — `perf_migration_start` needs the seeder's printed
timestamp. The CI/CD and LLM-eval datasets have no such dependency; order
doesn't matter for those two.

## What gets created

**CI/CD** (`*_cicd.tf`):
1. `cicd_build_p95` (query) — P95(duration_ms) of the build task, trailing
   window. What the trigger below watches.
2. `cicd_flaky_tests` (query) — COUNT of failed test-case spans by
   `test.name`. One row, always — the seeded flaky test.
3. `cicd_flaky_tests_by_branch` (query) — the same failures by
   `vcs.ref.head.name`. Two rows: the trunk and PR branches, differing in
   volume rather than rate, which is Chapter 18's blast-radius point.
4. `cicd_build_p95` (trigger) — fires when the build task's trailing P95
   exceeds `cicd_build_p95_threshold_ms` (default 15m, between the seeded
   ~12m baseline and ~22m regressed durations).

**Performance** (`*_perf.tf`):
1. `perf_slow_queries` — P99(duration_ms) by `db.query.text`. Workflow step 1.
2. `perf_query_heatmap` — HEATMAP(duration_ms), filtered to the bimodal
   query. Workflow step 2.
3. `perf_duration_by_user_type` — AVG(duration_ms) by `user.type`. Step 3.
4. `perf_duration_amd64` / `perf_duration_arm64` — P50 and P95, filtered by
   `host.arch`. Step 4 — the whole point is that these two look the same.
5. `graviton_migration` (marker) — the migration point. Doesn't delete on
   destroy, to preserve history.

**LLM** (`*_llm.tf`):
1. `llm_sli_good_response` (derived column) — MC4's exact SLI shape,
   `IF(GT($llm.response.quality_score, 0.7), 1, 0)`, reused against a
   different attribute.
2. `llm_sli` (query) — `AVG(sli_good_response)`. Below 1.0 is the seeded
   quality regression.

No Terraform here manages the real Claude Code telemetry — that's Collector
config (`../../session-1-fundamentals/collector/otel-collector-config.yaml`),
not a Honeycomb-side resource.
