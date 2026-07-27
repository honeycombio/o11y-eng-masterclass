# The deploy marker is what turns "latency looks off" into "latency looks off
# since 13:12", and it's the connective thread Chapter 2 keeps returning to:
# annotate deploys and every post-deploy query gets a free before/after
# boundary.
#
# Note the provider does not delete markers on destroy — that's deliberate on
# its part, to preserve history. Re-running with a new deploy_time adds another
# marker rather than moving this one.
resource "honeycombio_marker" "canary_deploy" {
  dataset = var.dataset

  message    = "deploy ${var.canary_version} (canary)"
  type       = "deploy"
  start_time = var.deploy_time
  url        = "https://github.com/honeycombio/o11y-eng-masterclass"
}
