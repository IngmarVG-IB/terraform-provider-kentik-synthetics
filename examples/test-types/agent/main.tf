# Agent-to-agent test, bidirectional, with a throughput task.
resource "kentik-synthetics_test" "agent" {
  name = "example-agent-to-agent"
  type = "agent"

  settings {
    agent {
      target     = "1717" # ID of the target agent
      reciprocal = true
    }
    agent_ids = ["968"]
    tasks     = ["ping", "traceroute", "throughput"]
    throughput {
      protocol = "tcp"
      port     = 5201
      duration = 10
    }
  }
}
