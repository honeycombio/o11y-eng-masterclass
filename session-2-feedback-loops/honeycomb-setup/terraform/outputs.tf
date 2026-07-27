output "board_url" {
  value       = honeycombio_flexible_board.weekly_review.board_url
  description = "Open this in Honeycomb to run the weekly review."
}

output "deploy_marker_time" {
  value       = var.deploy_time
  description = "Echoed back so you can confirm it matches what the seeder printed."
}
