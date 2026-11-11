# Network mesh: all listed agents test each other. The network_mesh block is optional.
resource "kentik-synthetics_test" "network_mesh" {
  name = "example-network-mesh"
  type = "network_mesh"

  settings {
    network_mesh {
      use_local_ip = true
    }
    agent_ids = ["968", "1717"]
    tasks     = ["ping", "traceroute"]
  }
}
