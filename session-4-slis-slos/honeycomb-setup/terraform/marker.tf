# Marks where the incident started, so the burn-window query and any
# before/after comparison have a boundary to read against — same role as the
# deploy marker in session 2, but here it marks an incident rather than a
# deploy, since this dataset has no single deploy event (see
# internal/scenario's package comment: the buggy build is an incomplete
# rollout, not a canary that shipped mid-window).
#
# The provider does not delete markers on destroy, to preserve history.
# Re-running the seeder and this apply with a fresh incident_start adds
# another marker rather than moving this one.
resource "honeycombio_marker" "incident_start" {
  dataset = var.dataset

  message    = "incident: elevated errors, ${var.buggy_version} + enterprise"
  type       = "incident"
  start_time = var.incident_start
  url        = "https://github.com/honeycombio/o11y-eng-masterclass"
}
