# DNS server grid: query the same name on several DNS servers.
resource "kentik-synthetics_test" "dns_grid" {
  name = "example-dns-grid"
  type = "dns_grid"

  settings {
    dns_grid {
      target      = "www.example.com"
      record_type = "DNS_RECORD_AAAA"
      servers     = ["192.0.2.53", "198.51.100.53"]
    }
    agent_ids = ["968", "1717"]
    tasks     = ["dns"]
  }
}
