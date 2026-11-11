# Notify two channels when the agent has not reported for 5 minutes.
resource "kentik-synthetics_agent_alert" "office_offline" {
  agent_id                 = "1717"
  threshold_seconds        = 300
  notification_channel_ids = ["1234", "5678"]
}
