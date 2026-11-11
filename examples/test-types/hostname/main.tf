# Ping and traceroute a host name from two agents, with grouped alerting.
resource "kentik-synthetics_test" "hostname" {
  name   = "example-hostname"
  type   = "hostname"
  status = "TEST_STATUS_ACTIVE"
  labels = ["example", "web"]

  settings {
    hostname {
      target = "www.example.com"
    }
    agent_ids             = ["968", "1717"]
    tasks                 = ["ping", "traceroute"]
    period                = 60
    family                = "IP_FAMILY_V4"
    notification_channels = ["1234"]
    notes                 = "Managed by Terraform"

    ping {
      protocol = "icmp"
      count    = 5
      timeout  = 3000
      delay    = 0
    }
    trace {
      protocol = "udp"
      port     = 33434
      count    = 3
      limit    = 30
      timeout  = 22500
    }
    health_settings {
      latency_critical     = 100000 # microseconds
      latency_warning      = 50000
      packet_loss_critical = 50 # percent
      packet_loss_warning  = 10
      jitter_critical      = 20000
      jitter_warning       = 10000
      activation {
        grace_period = "2"
        time_unit    = "m"
        time_window  = "5"
        times        = "3"
      }
    }
    alerting {
      alerting_type = "ALERTING_TYPE_GROUPED"
      grouped_alert_settings {
        default {
          metric               = "latency"
          src_group_by         = "SRC_GROUP_BY_ALL_AGENTS"
          percent_of_src_group = 50
        }
      }
    }
  }
}
