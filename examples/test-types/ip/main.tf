# Ping and traceroute a list of IP addresses.
resource "kentik-synthetics_test" "ip" {
  name = "example-ip"
  type = "ip"

  settings {
    ip {
      targets = ["192.0.2.1", "2001:db8::1"]
    }
    agent_ids = ["968"]
    tasks     = ["ping", "traceroute"]
    ping {
      protocol = "tcp"
      port     = 443
    }
  }
}
