# No hand-built UI URLs here, same reasoning as session 4's outputs.tf:
# honeycombio_query doesn't expose a computed link. Open the dataset's
# Queries page in the UI and find these by name.

output "cost_sampling_population_query_id" {
  value       = honeycombio_query.cost_sampling_population.id
  description = "Saved query: 'Cost/sampling: reconstructed request count'."
}

output "cost_sampling_by_route_query_id" {
  value       = honeycombio_query.cost_sampling_by_route.id
  description = "Saved query: 'Cost/sampling: reconstructed count by route'."
}
