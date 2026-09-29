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
  default     = 7200 # 2h
  description = <<-EOT
    Time range for both queries below. Wide enough to cover a seed run
    shortly before the session plus the ~26 minutes into the run of show
    where the demo queries it, without reaching back far enough to pick up
    an unrelated previous run. This is why the pre-session checklist seeds
    once, no earlier than an hour before going live.
  EOT
}
