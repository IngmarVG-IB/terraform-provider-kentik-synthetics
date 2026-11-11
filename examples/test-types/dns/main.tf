# DNS server monitor.
resource "kentik-synthetics_test" "dns" {
  name = "example-dns"
  type = "dns"

  settings {
    dns {
      target      = "www.example.com"
      record_type = "DNS_RECORD_A"
      servers     = ["192.0.2.53"]
      port        = 53
    }
    agent_ids = ["968"]
    tasks     = ["dns"]
    health_settings {
      dns_valid_codes = [0] # NOERROR
      dns_valid_ips   = "192.0.2.80"
    }
  }
}
