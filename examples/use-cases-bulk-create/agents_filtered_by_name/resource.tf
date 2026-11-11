// Create a test with list of agents that have a name containing given substring.
// Inputs: name substring

data "kentik-synthetics_agents" "agents" {}

locals {
  name_substring = "Comcast"
  agents_ids = [
    for agent in data.kentik-synthetics_agents.agents.items : agent.id
    if length(regexall(local.name_substring, agent.name)) > 0
  ]
}

resource "kentik-synthetics_test" "agents-filtered-by-name-test" {
  name = "agents-filtered-by-name-test"
  type = "hostname"
  settings {
    hostname {
      target = "www.example.com"
    }
    agent_ids = local.agents_ids
    tasks = [
      "ping",
      "traceroute"
    ]
    trace {
      protocol = "udp"
    }
    family = "IP_FAMILY_V4"
  }
}

output "agents-filtered-by-name-test-output" {
  value = kentik-synthetics_test.agents-filtered-by-name-test.id
}
