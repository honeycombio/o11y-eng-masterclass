variable "cicd_dataset" {
  type        = string
  default     = "cicd-pipeline"
  description = <<-EOT
    Dataset slug the CI/CD seeder writes into. Matches the service.name the
    seeder sets (cmd/seed-cicd-pipeline), since Honeycomb's OTLP ingest
    routes by that field.
  EOT
}

variable "cicd_burn_window_seconds" {
  type        = number
  default     = 3600 # 1h
  description = "Trailing window for the build-duration trigger."
}

variable "cicd_build_p95_threshold_ms" {
  type        = number
  default     = 900000 # 15m
  description = <<-EOT
    The trigger fires when P95(duration_ms) of the build task, over
    cicd_burn_window_seconds, exceeds this. Sits between the seeded baseline
    (~12m) and regressed (~22m) build durations — see
    internal/cicdscenario's package comment for the seeded numbers.
  EOT
}

variable "cicd_trigger_recipient_type" {
  type        = string
  default     = "email"
  description = "Recipient type for the build-duration trigger. See https://docs.honeycomb.io/notify/recipients/ for the full list."
}

variable "cicd_trigger_recipient_target" {
  type        = string
  description = <<-EOT
    Recipient target for the build-duration trigger — an email address,
    Slack channel, etc., matching cicd_trigger_recipient_type. No default:
    Honeycomb triggers require at least one recipient.
  EOT
}
