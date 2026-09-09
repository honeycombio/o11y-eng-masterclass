# Marks the Graviton migration point, same role as session 2's deploy
# marker: any before/after comparison needs a boundary to read against, even
# though the two comparison queries above filter by host.arch directly
# rather than by time.
#
# The provider does not delete markers on destroy, to preserve history.
resource "honeycombio_marker" "graviton_migration" {
  dataset = var.perf_dataset

  message    = "Graviton migration: amd64 -> arm64"
  type       = "deploy"
  start_time = var.perf_migration_start
  url        = "https://github.com/honeycombio/o11y-eng-masterclass"
}
