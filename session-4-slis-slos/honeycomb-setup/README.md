# Honeycomb setup for Masterclass 4

Everything the demo needs on the Honeycomb side, as Terraform. Unlike session
2, there's no shell-script alternative — a derived column plus a trigger with
a recipient block is enough surface area that hand-rolled `curl` calls would
just be more code to get subtly wrong.

**This is the free-tier workshop path**, not a native SLO. See
[`../README.md`](../README.md) for why: Honeycomb Free doesn't include
`honeycombio_slo` / `honeycombio_burn_alert`, so this builds the same SLI math
— a boolean derived column, `AVG()` over a scoped query, a threshold on a
trailing window — out of resources that work on every plan.

Run the seeder first regardless — it prints the incident-start timestamp this
Terraform needs.

## API keys

Two different keys are involved, and mixing them up is the most common
failure:

- The **Collector** uses a key with *send events* permission. That's the one
  in `HONEYCOMB_API_KEY` when you start the collector in
  [`../../session-1-fundamentals/collector`](../../session-1-fundamentals/collector).
- **This Terraform** needs a *configuration* key, with permission to manage
  derived columns, queries, triggers, and markers. A send-events key gets a 401
  here.

Honeycomb EU: export `HONEYCOMB_API_ENDPOINT=https://api.eu1.honeycomb.io`.

## Quickstart

```bash
cd ../..                                  # session-4-slis-slos
go run ./cmd/seed-sli-service             # prints the incident-start timestamp

cd honeycomb-setup/terraform
cp terraform.tfvars.example terraform.tfvars
$EDITOR terraform.tfvars                  # set incident_start from the seeder,
                                           # and trigger_recipient_target to a
                                           # real address — triggers require
                                           # at least one recipient

export HONEYCOMB_API_KEY=<config key>
terraform init
terraform plan
terraform apply
```

`terraform output trigger_id` and `terraform output burn_breakdown_query_id`
give you the IDs to find in the UI (there's no computed URL to output —
see `outputs.tf` for why).

The provider does not delete markers on destroy, to preserve incident history.
So `terraform destroy` cleans up the derived column, queries, and trigger but
leaves the marker behind; delete it in the UI for a clean slate. Because the
incident window is anchored to "now" at seed time (see the top-level README),
re-seeding for a second run means re-applying with a fresh `incident_start`
too — the old marker stays, a new one is added.

## What gets created

1. **`sli_good_request`** (`honeycombio_derived_column`) — the SLI itself:
   `IF(LT($http.response.status_code, 400), 1, 0)`. One boolean-as-1/0 column,
   reused by every query below.
2. **`baseline_sli`** (query) — `AVG(sli_good_request)` over a steady-state
   window, filtered to `route != '/healthz'`. The workshop's stand-in for a
   real SLO's 30-day baseline — see `variables.tf` for why the window here is
   hours, not days.
3. **`burn_trigger_sli`** (query) — the same `AVG()`, over the trailing burn
   window, no breakdown. Deliberately separate from the breakdown query below:
   a trigger needs one unambiguous number to threshold on.
4. **`burn_breakdown_sli`** (query) — the same trailing window, broken down by
   `service.version` and `user.type`, sorted so the worst row is first. This
   is what you click into once the trigger fires.
5. **`sli_burn`** (`honeycombio_trigger`) — fires when `burn_trigger_sli` drops
   below `burn_threshold` (default 0.90), checked every 5 minutes. The
   free-tier stand-in for a burn alert: a plain threshold on a recent window,
   not real error-budget burn-rate math, but the same shape — it reacts to
   what just happened, not to the dataset's lifetime average.
6. **`incident_start`** (`honeycombio_marker`) — marks where the incident
   began, so any before/after comparison has a boundary to read against.
