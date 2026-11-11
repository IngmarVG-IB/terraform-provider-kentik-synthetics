# Adopt an existing private agent (agents register themselves when installed; they cannot be created
# through the API), authorize it and manage its alias, site and labels.
resource "kentik-synthetics_agent" "office" {
  agent_id = "1717"
  status   = "AGENT_STATUS_OK" # authorize the waiting agent
  alias    = "office-ny-1"
  site_id  = "42"
  labels   = ["office", "us-east"]

  # Default false: `terraform destroy` only forgets the agent. Set to true to delete it from Kentik.
  delete_on_destroy = false
}
