# No hand-built UI URLs here, same reasoning as every prior session's
# outputs.tf: honeycombio_query and honeycombio_trigger don't expose a
# computed link.

output "cicd_build_p95_query_id" {
  value       = honeycombio_query.cicd_build_p95.id
  description = "Saved query: 'CI/CD: build task P95 duration, trailing window'."
}

output "cicd_flaky_tests_query_id" {
  value       = honeycombio_query.cicd_flaky_tests.id
  description = "Saved query: 'CI/CD: failed test cases by name'."
}

output "cicd_build_p95_trigger_id" {
  value       = honeycombio_trigger.cicd_build_p95.id
  description = "Trigger: 'CI/CD build regression: cicd-pipeline trailing window'."
}
