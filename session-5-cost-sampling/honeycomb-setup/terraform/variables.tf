variable "dataset" {
  type        = string
  default     = "cost-sampling-service"
  description = <<-EOT
    Dataset slug (not the display name) that the seeder writes into. Find it
    in the dataset's URL in the Honeycomb UI. Matches the service.name the
    seeder sets and the Collector's cost-sampling-service rule matches on
    (../../../session-1-fundamentals/collector/otel-collector-config.yaml),
    since Honeycomb's OTLP ingest routes by that field.
  EOT
}

variable "query_window_seconds" {
  type        = number
  default     = 1800 # 30m
  description = <<-EOT
    Time range for both queries below. Wide enough to comfortably cover a
    live seeding run (the default seed emits over roughly a minute, plus the
    Collector's decision_delay and adjustment_interval settling time) without
    reaching back far enough to pick up an unrelated previous run.
  EOT
}
