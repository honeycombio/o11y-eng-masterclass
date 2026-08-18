# No hand-built UI URLs here — unlike honeycombio_flexible_board (which
# exposes board_url), honeycombio_query and honeycombio_trigger don't expose
# a computed link, and guessing at one is exactly how a demo breaks live. Open
# the dataset's Queries or Triggers page in the UI and find these by the
# names/descriptions set in query.tf and trigger.tf instead.

output "baseline_sli_query_id" {
  value       = honeycombio_query.baseline_sli.id
  description = "Saved query: 'SLI: AVG success ratio (steady-state window)'."
}

output "burn_breakdown_query_id" {
  value       = honeycombio_query.burn_breakdown_sli.id
  description = "Saved query: 'SLI: AVG success ratio, trailing burn window, by version/plan'. Open this once the trigger fires."
}

output "trigger_id" {
  value       = honeycombio_trigger.sli_burn.id
  description = "Trigger: 'SLI burn: sli-demo-service trailing window'. Open it in the Honeycomb UI (Triggers) to watch it evaluate live."
}

output "incident_start_time" {
  value       = var.incident_start
  description = "Echoed back so you can confirm it matches what the seeder printed."
}
