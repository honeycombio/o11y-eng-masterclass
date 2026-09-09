variable "llm_eval_dataset" {
  type        = string
  default     = "llm-eval-pipeline"
  description = <<-EOT
    Dataset slug the LLM-eval seeder writes into. Matches the service.name
    the seeder sets (cmd/seed-llm-eval-pipeline).
  EOT
}

variable "llm_eval_query_window_seconds" {
  type        = number
  default     = 86400 # 24h, matching the seeder's default Window
  description = "Time range for the SLI query."
}
