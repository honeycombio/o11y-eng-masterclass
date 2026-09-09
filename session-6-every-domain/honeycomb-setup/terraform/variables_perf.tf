variable "perf_dataset" {
  type        = string
  default     = "perf-service"
  description = <<-EOT
    Dataset slug the performance seeder writes into. Matches the service.name
    the seeder sets (cmd/seed-perf-service).
  EOT
}

variable "perf_query_window_seconds" {
  type        = number
  default     = 172800 # 48h, matching the seeder's default Window
  description = "Time range for the P99-by-query and AVG-by-user-type queries — wide enough to cover the whole seeded window."
}

variable "perf_migration_start" {
  type        = number
  description = <<-EOT
    Unix timestamp, in seconds, that seed-perf-service printed as the
    migration completion time. Used to scope the amd64-only and arm64-only
    before/after comparison queries. No default: must match the seeded data
    or the two queries won't line up with the actual migration point.
  EOT

  validation {
    condition     = var.perf_migration_start > 1000000000 && var.perf_migration_start < 10000000000
    error_message = "perf_migration_start must be a Unix timestamp in seconds, not milliseconds."
  }
}
