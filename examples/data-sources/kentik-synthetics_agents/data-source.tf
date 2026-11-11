# All agents within 1000 km of Amsterdam.
data "kentik-synthetics_agents" "near_amsterdam" {
  latitude     = 52.37
  longitude    = 4.89
  max_distance = 1000
}

output "agent_ids_near_amsterdam" {
  value = [for a in data.kentik-synthetics_agents.near_amsterdam.items : a.id]
}
