variable "dataset" {
  type        = string
  default     = "sli-demo-service"
  description = <<-EOT
    Dataset slug (not the display name) that the seeder writes into. Find it
    in the dataset's URL in the Honeycomb UI. Matches the service.name the
    seeder sets, since Honeycomb's OTLP ingest routes by that field.
  EOT
}

variable "healthy_version" {
  type        = string
  default     = "1.5.0"
  description = "service.version value most traffic runs. Must match the seeder's scenario.Config.HealthyVersion."
}

variable "buggy_version" {
  type        = string
  default     = "1.4.2"
  description = "service.version value the incident is scoped to. Must match the seeder's scenario.Config.BuggyVersion."
}

variable "baseline_window_seconds" {
  type        = number
  default     = 21600 # 6h, matching the seeder's default Window
  description = <<-EOT
    Time range for the steady-state SLI query. Real Honeycomb SLOs baseline
    against 28 days; this seeds only a 6-hour window (see internal/scenario's
    package comment for why), so the workshop's "baseline" query is scoped to
    match the data that actually exists rather than a mostly-empty 28 days.
  EOT
}

variable "burn_window_seconds" {
  type        = number
  default     = 3600 # 1h
  description = "Trailing window for the burn query and the trigger — the fast-burn half of the SRE Workbook's multi-window pairing, which is the shape Chapter 12 generalises as a relative burn alert."
}

variable "burn_threshold" {
  type        = number
  default     = 0.90
  description = <<-EOT
    The trigger fires when AVG() of the SLI derived column drops below this
    over burn_window_seconds. 0.90 sits comfortably below the ~0.95+ the
    dataset's non-incident traffic holds and comfortably above the ~0.5-0.6
    the incident window drops to — see internal/scenario's package comment for
    the seeded numbers.
  EOT
}

variable "trigger_recipient_type" {
  type        = string
  default     = "email"
  description = "Recipient type for the burn trigger. See https://docs.honeycomb.io/notify/recipients/ for the full list (email, slack, pagerduty, webhook, msteams, ...)."
}

variable "trigger_recipient_target" {
  type        = string
  description = <<-EOT
    Recipient target for the burn trigger — an email address, Slack channel,
    etc., matching trigger_recipient_type. No default: Honeycomb triggers
    require at least one recipient, and there is no reasonable placeholder
    that wouldn't silently notify a stranger.
  EOT
}

variable "incident_start" {
  type        = number
  description = <<-EOT
    Unix timestamp, in seconds, that seed-sli-service printed as the incident
    window's start. Must match the seeded data or the marker (and any
    before/after read against it) will not line up.
  EOT

  validation {
    # Sanity bound rather than a real range check: catches milliseconds pasted
    # in place of seconds, which is the mistake that actually happens.
    condition     = var.incident_start > 1000000000 && var.incident_start < 10000000000
    error_message = "incident_start must be a Unix timestamp in seconds, not milliseconds."
  }
}
