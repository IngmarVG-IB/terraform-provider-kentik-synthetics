data "kentik-synthetics_agents" "agents" {}

output "agent_ids" {
  value = [for a in data.kentik-synthetics_agents.agents.items : a.id]
}

data "kentik-synthetics_agent" "agent" {
  id = "968" # ID of an existing agent
}

output "agent_alias" {
  value = data.kentik-synthetics_agent.agent.alias
}

data "kentik-synthetics_tests" "tests" {}

output "test_names" {
  value = [for t in data.kentik-synthetics_tests.tests.items : t.name]
}

data "kentik-synthetics_test" "test" {
  id = "3271" # ID of an existing test
}

output "test_settings" {
  value = data.kentik-synthetics_test.test.settings
}
