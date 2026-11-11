# Alerts of two agents; omit agent_ids to list all agent alerts.
data "kentik-synthetics_agent_alerts" "office" {
  agent_ids = ["968", "1717"]
}

output "office_alert_ids" {
  value = [for a in data.kentik-synthetics_agent_alerts.office.items : a.id]
}
