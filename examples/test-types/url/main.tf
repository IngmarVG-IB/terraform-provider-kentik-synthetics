# HTTP(S) / API test, created paused.
resource "kentik-synthetics_test" "url" {
  name   = "example-url"
  type   = "url"
  status = "paused" # same as TEST_STATUS_PAUSED

  settings {
    url {
      target  = "https://www.example.com/health"
      method  = "GET"
      timeout = 5000
      headers = {
        "Accept" = "application/json"
      }
      ignore_tls_errors = false
    }
    agent_ids = ["968"]
    tasks     = ["http", "ping", "traceroute"]
    health_settings {
      http_latency_critical = 500000
      http_latency_warning  = 250000
      http_valid_codes      = [200, 204]
      cert_expiry_warning   = 30 # days
      cert_expiry_critical  = 7
    }
  }
}
