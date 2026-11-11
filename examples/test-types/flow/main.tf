# Autonomous (flow based) test: targets are selected from flow data for an ASN.
resource "kentik-synthetics_test" "flow" {
  name = "example-flow-asn"
  type = "flow"

  settings {
    flow {
      type                           = "asn"
      target                         = "15169"
      direction                      = "dst"
      inet_direction                 = "src"
      max_ip_targets                 = 10
      max_providers                  = 3
      target_refresh_interval_millis = 43200000
    }
    agent_ids = ["968"]
    tasks     = ["ping", "traceroute"]
  }
}
