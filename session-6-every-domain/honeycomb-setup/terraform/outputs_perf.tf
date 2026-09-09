output "perf_slow_queries_query_id" {
  value       = honeycombio_query.perf_slow_queries.id
  description = "Saved query: 'Perf: P99 duration by query'."
}

output "perf_query_heatmap_query_id" {
  value       = honeycombio_query.perf_query_heatmap.id
  description = "Saved query: 'Perf: duration heatmap, orders-by-status query'."
}

output "perf_duration_by_user_type_query_id" {
  value       = honeycombio_query.perf_duration_by_user_type.id
  description = "Saved query: 'Perf: AVG duration by user.type'."
}

output "perf_duration_amd64_query_id" {
  value       = honeycombio_query.perf_duration_amd64.id
  description = "Saved query: 'Perf: P50/P95 duration, amd64 (pre-migration)'."
}

output "perf_duration_arm64_query_id" {
  value       = honeycombio_query.perf_duration_arm64.id
  description = "Saved query: 'Perf: P50/P95 duration, arm64 (post-migration)'."
}
