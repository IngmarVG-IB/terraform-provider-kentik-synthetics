# Network grid: every agent tests every target.
resource "kentik-synthetics_test" "network_grid" {
  name = "example-network-grid"
  type = "network_grid"

  settings {
    network_grid {
      targets = ["192.0.2.1", "192.0.2.2", "192.0.2.3"]
    }
    agent_ids = ["968", "1717"]
    tasks     = ["ping", "traceroute"]
  }
}
