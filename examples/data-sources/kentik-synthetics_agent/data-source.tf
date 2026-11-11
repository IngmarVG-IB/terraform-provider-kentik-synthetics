data "kentik-synthetics_agent" "amsterdam" {
  id = "968"
}

output "amsterdam_agent_site" {
  value = data.kentik-synthetics_agent.amsterdam.site_name
}
