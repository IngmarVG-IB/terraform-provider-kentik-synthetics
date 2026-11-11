data "kentik-synthetics_agent_alert" "existing" {
  id = "9876"
}

output "existing_alert_threshold" {
  value = data.kentik-synthetics_agent_alert.existing.threshold_seconds
}
