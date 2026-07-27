variable "dataset" {
  type        = string
  default     = "sample-service"
  description = <<-EOT
    Dataset slug (not the display name) that the seeders write into. Find it in
    the dataset's URL in the Honeycomb UI.
  EOT
}

variable "deploy_time" {
  type        = number
  description = <<-EOT
    Unix timestamp, in seconds, of the canary deploy. This must match the value
    seed-canary-regression printed, or the marker will not line up with the
    data and the before/after comparison in the demo falls apart.
  EOT

  validation {
    # Sanity bound rather than a real range check: catches milliseconds pasted
    # in place of seconds, which is the mistake that actually happens.
    condition     = var.deploy_time > 1000000000 && var.deploy_time < 10000000000
    error_message = "deploy_time must be a Unix timestamp in seconds, not milliseconds."
  }
}

variable "canary_version" {
  type        = string
  default     = "1.5.0"
  description = "service.version value the canary build reports."
}

variable "baseline_version" {
  type        = string
  default     = "1.4.2"
  description = "service.version value the pre-deploy build reports."
}

variable "review_window_seconds" {
  type        = number
  default     = 14400 # 4h, matching the seeder's default window
  description = "Time range for the board's queries, in seconds."
}
