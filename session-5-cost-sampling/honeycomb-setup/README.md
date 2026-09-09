# Honeycomb setup for Masterclass 5

Everything the demo needs on the Honeycomb side, as Terraform — two saved
queries, no derived column, no trigger, no marker. Unlike session 4, there's
no free-tier gate to work around here at all: this demo is plain `COUNT()`,
which every plan supports.

## API keys

Two different keys are involved, same split as every other session:

- The **Collector** uses a key with *send events* permission. That's the one
  in `HONEYCOMB_API_KEY` when you start the collector in
  [`../../session-1-fundamentals/collector`](../../session-1-fundamentals/collector).
- **This Terraform** needs a *configuration* key, with permission to manage
  queries. A send-events key gets a 401 here.

Honeycomb EU: export `HONEYCOMB_API_ENDPOINT=https://api.eu1.honeycomb.io`.

## Quickstart

```bash
export HONEYCOMB_API_KEY=<config key>
cd terraform
terraform init
terraform plan
terraform apply
```

`terraform output cost_sampling_population_query_id` and
`cost_sampling_by_route_query_id` give you the IDs to find in the UI (there's
no computed URL to output — see `outputs.tf` for why).

Apply this before or after seeding — order doesn't matter, unlike session 4.
There's no incident timestamp to thread through; both queries just read
`var.query_window_seconds` back from now.

## What gets created

1. **`cost_sampling_population`** (query) — `COUNT()`, no breakdown, over the
   whole dataset. Compare its number to the seeder's own printed "seeded N
   requests" — they should land close, even though only a fraction of that N
   is physically stored. That gap, and the fact the two numbers still agree,
   is the entire demo.
2. **`cost_sampling_by_route`** (query) — the same `COUNT()`, broken down by
   `http.route`, sorted descending. Confirms the Collector's per-key
   `adaptive_percentage` sampler kept `/api/admin/report`'s 1% share visible
   rather than a single global rate swallowing it into noise.

See [`../README.md`](../README.md) for the live demo run order and
[`../../session-1-fundamentals/collector/otel-collector-config.yaml`](../../session-1-fundamentals/collector/otel-collector-config.yaml)
for the Collector-side rules these queries are reading the effect of.
