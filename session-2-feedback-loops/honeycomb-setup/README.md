# Honeycomb setup for Masterclass 2

Everything the demo needs on the Honeycomb side, in two forms. Both create the
same objects — pick whichever suits you.

| | [`scripts/`](scripts) | [`terraform/`](terraform) |
|---|---|---|
| Install needed | none, just `curl` | Terraform ≥ 1.5 |
| Creates | the deploy marker | marker, four saved queries, the weekly-review board |
| Best for | following along quickly | keeping the setup, and observability-as-code as its own lesson |

Run the seeder first either way — it prints the deploy timestamp both paths
need.

## API keys

Two different keys are involved, and mixing them up is the most common failure:

- The **Collector** uses a key with *send events* permission. That's the one in
  `HONEYCOMB_API_KEY` when you start the collector in
  [`../../session-1-fundamentals/collector`](../../session-1-fundamentals/collector).
- **These scripts and the Terraform config** need a *configuration* key, with
  permission to manage markers, queries, and boards. A send-events key gets a
  401 here.

Honeycomb EU: export `HONEYCOMB_API_ENDPOINT=https://api.eu1.honeycomb.io`.
Both paths respect it.

## Quickstart: shell

```bash
cd ../..                                  # session-2-feedback-loops
go run ./cmd/seed-canary-regression       # prints the deploy timestamp

cd honeycomb-setup/scripts
HONEYCOMB_API_KEY=<config key> ./create-deploy-marker.sh <timestamp>
```

## Terraform

```bash
cd terraform
cp terraform.tfvars.example terraform.tfvars
$EDITOR terraform.tfvars                  # set deploy_time from the seeder

export HONEYCOMB_API_KEY=<config key>
terraform init
terraform plan
terraform apply
```

`terraform output board_url` gives you the board to open.

The provider does not delete markers on destroy — that's intentional on its
part, to preserve deploy history. So `terraform destroy` cleans up the board and
queries but leaves the marker behind; delete it in the UI if you want a clean
slate. Re-applying with a new `deploy_time` adds another marker rather than
moving the existing one, which matters if you re-seed.

## What the board contains

The four standing queries of Chapter 24's weekly observability review, which are
also the demo's path through the core analysis loop:

1. **P95 latency by build** — did the deploy move latency? The deploy marker
   plus this graph answers the first question of any post-deploy investigation.
2. **Billing latency distribution** — a heatmap, because a second band of slow
   requests is visible here even when the percentile line barely moves. This is
   where you draw the BubbleUp box.
3. **Billing P95 by plan** — for whom. Enterprise diverging from free and
   premium is the signal.
4. **Top error patterns** — what's failing, and is the mix changing.

All four filter to root spans (`trace.parent_id does-not-exist`). Without that,
percentiles get computed across child spans too and drift toward whatever the
fastest step happens to be.
