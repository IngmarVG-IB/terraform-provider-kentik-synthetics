---
page_title: "kentik-synthetics_test Resource - kentik-synthetics"
subcategory: ""
description: |-
  Synthetic test (synthetics API v202309). The type selects which settings block configures the test target (for example type = "hostname" with settings { hostname { ... } }). Supported types: hostname, ip, network_grid, agent, network_mesh, flow, url, page_load, dns, dns_grid. Tests can be imported by ID: terraform import kentik-synthetics_test.example <id>.
---

# kentik-synthetics_test (Resource)

Synthetic test (synthetics API v202309). The `type` selects which `settings` block configures the test target (for example `type = "hostname"` with `settings { hostname { ... } }`). Supported types: `hostname`, `ip`, `network_grid`, `agent`, `network_mesh`, `flow`, `url`, `page_load`, `dns`, `dns_grid`. Tests can be imported by ID: `terraform import kentik-synthetics_test.example <id>`.

## Example usage

```terraform
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
```

## Examples for every test type

### `ip`

```terraform
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
```

### `network_grid`

```terraform
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
```

### `agent`

```terraform
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
```

### `network_mesh`

```terraform
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
```

### `flow`

```terraform
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
```

### `url`

```terraform
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
```

### `page_load`

```terraform
# Page load test with headless Chromium (requires an app agent).
resource "kentik-synthetics_test" "page_load" {
  name = "example-page-load"
  type = "page_load"

  settings {
    page_load {
      target  = "https://www.example.com/"
      timeout = 10000
      css_selectors = {
        "logo" = "#logo"
      }
    }
    agent_ids = ["1717"]
    tasks     = ["page-load"]
  }
}
```

### `dns`

```terraform
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
```

### `dns_grid`

```terraform
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
```

## Status

`status` is applied with the SetTestStatus API call, so pausing or resuming a test does not re-create it
and does not touch its configuration.

## Deprecated attributes

Attributes of the former v202101beta1 API are still accepted and marked deprecated. Where there is a
v202309 equivalent the value is used for it (for example `settings.protocol` for `settings.ping.protocol`);
otherwise it is ignored. The upgrade guide lists the mapping.

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `name` (String) User selected name of the test.
- `settings` (Block List, Min: 1, Max: 1) Test configuration. (see [below for nested schema](#nestedblock--settings))
- `type` (String) Type of the test. Changing it forces a new test; the matching `settings` block (for example `settings.hostname` for `hostname`) must be set.

### Optional

- `labels` (Set of String) Set of labels associated with the test.
- `status` (String) Operational status of the test: `TEST_STATUS_ACTIVE` (default) or `TEST_STATUS_PAUSED`; `active` and `paused` are accepted too. Changes are applied with the SetTestStatus API call, without re-creating the test.

### Read-Only

- `cdate` (String) Creation timestamp (UTC).
- `created_by` (List of Object) Identity of test creator. (see [below for nested schema](#nestedatt--created_by))
- `edate` (String) Last modification timestamp (UTC).
- `expires_on` (String, Deprecated) **Deprecated.** Not provided by synthetics API v202309; always empty.
- `id` (String) ID of the test.
- `last_updated_by` (List of Object) Identity of use that has modified the test last. (see [below for nested schema](#nestedatt--last_updated_by))

<a id="nestedblock--settings"></a>
### Nested Schema for `settings`

Required:

- `agent_ids` (List of String) IDs of agents assigned to run tasks on behalf of the test.
- `tasks` (List of String) List of task names to run for the test: `ping`, `traceroute`, `http`, `page-load`, `dns`, `throughput`. Which tasks are valid depends on the test type.

Optional:

- `agent` (Block List, Max: 1) Target definition of a `agent` test. Required when `type = "agent"` and not allowed otherwise. (see [below for nested schema](#nestedblock--settings--agent))
- `alerting` (Block List, Max: 1) Alerting configuration parameters. (see [below for nested schema](#nestedblock--settings--alerting))
- `count` (Number, Deprecated) **Deprecated.** Ignored. Use `ping.count` / `trace.count` instead.
- `dns` (Block List, Max: 1) Target definition of a `dns` test. Required when `type = "dns"` and not allowed otherwise. (see [below for nested schema](#nestedblock--settings--dns))
- `dns_grid` (Block List, Max: 1) Target definition of a `dns_grid` test. Required when `type = "dns_grid"` and not allowed otherwise. (see [below for nested schema](#nestedblock--settings--dns_grid))
- `expiry` (Number, Deprecated) **Deprecated.** Ignored. Use `ping.timeout` / `trace.timeout` instead.
- `family` (String) IP address family to select from available DNS name resolutions. Valid values: `IP_FAMILY_V4`, `IP_FAMILY_V6`, `IP_FAMILY_DUAL`.
- `flow` (Block List, Max: 1) Target definition of a `flow` test. Required when `type = "flow"` and not allowed otherwise. (see [below for nested schema](#nestedblock--settings--flow))
- `health_settings` (Block List, Max: 1) Health evaluation thresholds, acceptable responses and alarm activation settings. (see [below for nested schema](#nestedblock--settings--health_settings))
- `hostname` (Block List, Max: 1) Target definition of a `hostname` test. Required when `type = "hostname"` and not allowed otherwise. (see [below for nested schema](#nestedblock--settings--hostname))
- `http` (Block List, Max: 1, Deprecated) **Deprecated.** Use the `url` or `page_load` block instead. (see [below for nested schema](#nestedblock--settings--http))
- `ip` (Block List, Max: 1) Target definition of a `ip` test. Required when `type = "ip"` and not allowed otherwise. (see [below for nested schema](#nestedblock--settings--ip))
- `limit` (Number, Deprecated) **Deprecated.** Ignored. Use `trace.limit` instead.
- `monitoring_settings` (Block List, Max: 1, Deprecated) **Deprecated.** Use `settings.notification_channels` instead. (see [below for nested schema](#nestedblock--settings--monitoring_settings))
- `network_grid` (Block List, Max: 1) Target definition of a `network_grid` test. Required when `type = "network_grid"` and not allowed otherwise. (see [below for nested schema](#nestedblock--settings--network_grid))
- `network_mesh` (Block List, Max: 1) Target definition of a `network_mesh` test (all agents in `agent_ids` test each other). Optional when `type = "network_mesh"` and not allowed otherwise. (see [below for nested schema](#nestedblock--settings--network_mesh))
- `notes` (String) Add a note or comment for this test.
- `notification_channels` (List of String) List of IDs of notification channels for alarms triggered by the test.
- `page_load` (Block List, Max: 1) Target definition of a `page_load` test. Required when `type = "page_load"` and not allowed otherwise. (see [below for nested schema](#nestedblock--settings--page_load))
- `period` (Number) Test evaluation period (in seconds).
- `ping` (Block List, Max: 1) Ping tasks configuration parameters. (see [below for nested schema](#nestedblock--settings--ping))
- `port` (Number, Deprecated) **Deprecated.** Use `ping.port` instead.
- `protocol` (String, Deprecated) **Deprecated.** Use `ping.protocol` instead.
- `rollup_level` (Number, Deprecated) **Deprecated.** Ignored by synthetics API v202309.
- `schedule` (Block List, Max: 1) Scheduled test configuration parameters. (see [below for nested schema](#nestedblock--settings--schedule))
- `servers` (List of String, Deprecated) **Deprecated.** Use `dns.servers` / `dns_grid.servers` instead.
- `site` (Block List, Max: 1, Deprecated) **Deprecated.** `site` tests are not supported by synthetics API v202309 and are ignored. (see [below for nested schema](#nestedblock--settings--site))
- `tag` (Block List, Max: 1, Deprecated) **Deprecated.** `tag` tests are not supported by synthetics API v202309 and are ignored. (see [below for nested schema](#nestedblock--settings--tag))
- `throughput` (Block List, Max: 1) Throughput task configuration parameters. (see [below for nested schema](#nestedblock--settings--throughput))
- `trace` (Block List, Max: 1) Traceroute task configuration parameters. (see [below for nested schema](#nestedblock--settings--trace))
- `url` (Block List, Max: 1) Target definition of a `url` test. Required when `type = "url"` and not allowed otherwise. (see [below for nested schema](#nestedblock--settings--url))

<a id="nestedblock--settings--agent"></a>
### Nested Schema for `settings.agent`

Required:

- `target` (String) ID of the target agent.

Optional:

- `reciprocal` (Boolean) Boolean value indicating whether to make the test bidirectional.
- `use_local_ip` (Boolean) Boolean value indicating whether to use local (private) IP address of the target agent.


<a id="nestedblock--settings--alerting"></a>
### Nested Schema for `settings.alerting`

Optional:

- `alerting_type` (String) Alerting type for the test: per-agent (ALERTING_TYPE_AGENT) or grouped (ALERTING_TYPE_GROUPED). Valid values: `ALERTING_TYPE_AGENT`, `ALERTING_TYPE_GROUPED`, `ALERTING_TYPE_SUBTEST`.
- `disable_warning_notifications` (Boolean) Boolean indicating whether to disable warning-level alert notifications.
- `grouped_alert_settings` (Block List, Max: 1) Grouped alerting settings. (see [below for nested schema](#nestedblock--settings--alerting--grouped_alert_settings))

<a id="nestedblock--settings--alerting--grouped_alert_settings"></a>
### Nested Schema for `settings.alerting.grouped_alert_settings`

Optional:

- `default` (Block List, Max: 1) Default grouped alerting settings. (see [below for nested schema](#nestedblock--settings--alerting--grouped_alert_settings--default))
- `overrides` (Block List) Overrides to default grouped alerting settings. (see [below for nested schema](#nestedblock--settings--alerting--grouped_alert_settings--overrides))

<a id="nestedblock--settings--alerting--grouped_alert_settings--default"></a>
### Nested Schema for `settings.alerting.grouped_alert_settings.default`

Optional:

- `filter_ids` (List of Number) List of IDs to include in the group.
- `metric` (String) Metric.
- `percent_of_src_group` (Number) Grouping percentage.
- `src_group_by` (String) Grouping. Valid values: `SRC_GROUP_BY_ALL_AGENTS`, `SRC_GROUP_BY_LABEL`, `SRC_GROUP_BY_SITE`.


<a id="nestedblock--settings--alerting--grouped_alert_settings--overrides"></a>
### Nested Schema for `settings.alerting.grouped_alert_settings.overrides`

Optional:

- `filter_ids` (List of Number) List of IDs to include in the group.
- `metric` (String) Metric.
- `percent_of_src_group` (Number) Grouping percentage.
- `src_group_by` (String) Grouping. Valid values: `SRC_GROUP_BY_ALL_AGENTS`, `SRC_GROUP_BY_LABEL`, `SRC_GROUP_BY_SITE`.




<a id="nestedblock--settings--dns"></a>
### Nested Schema for `settings.dns`

Required:

- `target` (String) Fully qualified DNS name to query.

Optional:

- `port` (Number) Target DNS server port.
- `record_type` (String) Type of DNS record to query. Valid values: `DNS_RECORD_A`, `DNS_RECORD_AAAA`, `DNS_RECORD_CNAME`, `DNS_RECORD_DNAME`, `DNS_RECORD_NS`, `DNS_RECORD_MX`, `DNS_RECORD_PTR`, `DNS_RECORD_SOA`.
- `servers` (List of String) List of IP addresses of DNS servers.
- `timeout` (Number, Deprecated) **Deprecated.** Ignored by the Kentik API.
- `type` (String, Deprecated) **Deprecated.** Use `record_type` instead.


<a id="nestedblock--settings--dns_grid"></a>
### Nested Schema for `settings.dns_grid`

Optional:

- `port` (Number) Target DNS server port.
- `record_type` (String) Type of DNS record to query. Valid values: `DNS_RECORD_A`, `DNS_RECORD_AAAA`, `DNS_RECORD_CNAME`, `DNS_RECORD_DNAME`, `DNS_RECORD_NS`, `DNS_RECORD_MX`, `DNS_RECORD_PTR`, `DNS_RECORD_SOA`.
- `servers` (List of String) List of IP addresses of DNS servers.
- `target` (String) Fully qualified DNS name to query.
- `targets` (List of String, Deprecated) **Deprecated.** Use `target` (a single DNS name) and `servers` instead. Only the first element is used.
- `timeout` (Number, Deprecated) **Deprecated.** Ignored by the Kentik API.
- `type` (String, Deprecated) **Deprecated.** Use `record_type` instead.


<a id="nestedblock--settings--flow"></a>
### Nested Schema for `settings.flow`

Required:

- `target` (String) Target ASN, CDN, Country, Region of City for autonomous test (type of value depends on flow test sub-type).
- `type` (String) Autonomous test sub-type (asn | cdn | country | region | city). Valid values: `asn`, `cdn`, `country`, `region`, `city`.

Optional:

- `direction` (String) Direction of flows to match target attribute for extraction of target addresses (src | dst).
- `inet_direction` (String) Selection of address from flow data (src = source address in inbound flows | dst = destination addresses in outbound flows).
- `max_ip_targets` (Number) Maximum number of target IP addresses to select based flow data query.
- `max_providers` (Number) Maximum number of IP providers to track autonomously.
- `max_tasks` (Number, Deprecated) **Deprecated.** Use `max_ip_targets` instead.
- `target_refresh_interval_millis` (Number) Period (in milliseconds) for refreshing list of targets based on available flow data.


<a id="nestedblock--settings--health_settings"></a>
### Nested Schema for `settings.health_settings`

Optional:

- `activation` (Block List, Max: 1) Alarm activation settings. (see [below for nested schema](#nestedblock--settings--health_settings--activation))
- `cert_expiry_critical` (Number) Threshold for remaining validity of TLS certificate (in days) to trigger critical alarm.
- `cert_expiry_warning` (Number) Threshold for remaining validity of TLS certificate (in days) to trigger warning alarm.
- `disable_alerts` (Boolean) Boolean value indicating whether to disable all alerts for this test.
- `disabled_metrics` (Block List, Max: 1) Set of metrics to be skipped during health evaluation. (see [below for nested schema](#nestedblock--settings--health_settings--disabled_metrics))
- `dns_latency_critical` (Number) Threshold for DNS response latency (in microseconds) to trigger critical alarm.
- `dns_latency_critical_stddev` (Number) Threshold for standard deviation (in microseconds) of DNS response latency to trigger critical alarm.
- `dns_latency_warning` (Number) Threshold for DNS response latency (in microseconds) to trigger warning alarm.
- `dns_latency_warning_stddev` (Number) Threshold for standard deviation (in microseconds) of DNS response latency to trigger warning alarm.
- `dns_valid_codes` (List of Number) List of DNS status codes indicating healthy state.
- `dns_valid_ips` (String) Comma separated list of IP addresses expected to be received in response to DNS A or AAAA query.
- `health_disabled` (Boolean) Disable all health evaluation for this test.
- `http_latency_critical` (Number) Threshold for HTTP response latency (in microseconds) to trigger critical alarm.
- `http_latency_critical_stddev` (Number) Threshold for standard deviation of HTTP response latency (in microseconds) to trigger critical alarm.
- `http_latency_warning` (Number) Threshold for HTTP response latency (in microseconds) to trigger warning alarm.
- `http_latency_warning_stddev` (Number) Threshold for standard deviation of HTTP response latency (in microseconds) to trigger warning alarm.
- `http_valid_codes` (List of Number) List of HTTP status codes indicating healthy state.
- `jitter_critical` (Number) Threshold for ping jitter (in microseconds) to trigger critical alarm.
- `jitter_critical_stddev` (Number) Threshold for standard deviation of ping jitter (in microseconds) to trigger critical alarm.
- `jitter_warning` (Number) Threshold for ping jitter (in microseconds) to trigger critical alarm.
- `jitter_warning_stddev` (Number) Threshold for standard deviation of ping jitter (in microseconds) to trigger warning alarm.
- `latency_critical` (Number) Threshold for ping response latency (in microseconds) to trigger critical alarm.
- `latency_critical_stddev` (Number) Threshold for standard deviation (in microseconds) of ping response latency to trigger critical alarm.
- `latency_warning` (Number) Threshold for ping response latency (in microseconds) to trigger warning alarm.
- `latency_warning_stddev` (Number) Threshold for standard deviation (in microseconds) of ping response latency to trigger warning alarm.
- `packet_loss_critical` (Number) Threshold for ping packet loss (in %) to trigger critical alarm.
- `packet_loss_warning` (Number) Threshold for ping packet loss (in %) to trigger warning alarm.
- `throughput_critical` (Number) Threshold for throughput bandwidth (in mbps) to trigger critical alarm.
- `throughput_critical_stddev` (Number) Threshold for standard deviation (in mbps) of throughput bandwidth to trigger critical alarm.
- `throughput_warning` (Number) Threshold for throughput bandwidth (in mbps) to trigger warning alarm.
- `throughput_warning_stddev` (Number) Threshold for standard deviation (in mbps) of throughput bandwidth to trigger warning alarm.

<a id="nestedblock--settings--health_settings--activation"></a>
### Nested Schema for `settings.health_settings.activation`

Optional:

- `grace_period` (String) Period of healthy status in minutes within the time window not cancelling alarm activation.
- `time_unit` (String) Time unit for specifying time window (m | h).
- `time_window` (String) Time window for evaluating of test for alarm activation.
- `times` (String) Number of occurrences of unhealthy test status within the time window triggering alarm activation.


<a id="nestedblock--settings--health_settings--disabled_metrics"></a>
### Nested Schema for `settings.health_settings.disabled_metrics`

Optional:

- `dns_codes` (Boolean) Skip `dns_codes` during health evaluation.
- `dns_ips` (Boolean) Skip `dns_ips` during health evaluation.
- `dns_latency` (Boolean) Skip `dns_latency` during health evaluation.
- `http_cert_expiry` (Boolean) Skip `http_cert_expiry` during health evaluation.
- `http_codes` (Boolean) Skip `http_codes` during health evaluation.
- `http_headers` (Boolean) Skip `http_headers` during health evaluation.
- `http_latency` (Boolean) Skip `http_latency` during health evaluation.
- `ping_jitter` (Boolean) Skip `ping_jitter` during health evaluation.
- `ping_latency` (Boolean) Skip `ping_latency` during health evaluation.
- `ping_packet_loss` (Boolean) Skip `ping_packet_loss` during health evaluation.
- `throughput_bandwidth` (Boolean) Skip `throughput_bandwidth` during health evaluation.
- `transaction_latency` (Boolean) Skip `transaction_latency` during health evaluation.



<a id="nestedblock--settings--hostname"></a>
### Nested Schema for `settings.hostname`

Required:

- `target` (String) Fully qualified DNS name of the target host.


<a id="nestedblock--settings--http"></a>
### Nested Schema for `settings.http`

Optional:

- `body` (String, Deprecated) **Deprecated.** Use `url.body` instead.
- `css_selectors` (Map of String, Deprecated) **Deprecated.** Use `page_load.css_selectors` instead.
- `expiry` (Number, Deprecated) **Deprecated.** Use `url.timeout` / `page_load.timeout` instead.
- `headers` (Map of String, Deprecated) **Deprecated.** Use `url.headers` / `page_load.headers` instead.
- `ignore_tls_errors` (Boolean, Deprecated) **Deprecated.** Use `url.ignore_tls_errors` / `page_load.ignore_tls_errors` instead.
- `method` (String, Deprecated) **Deprecated.** Use `url.method` instead.
- `period` (Number, Deprecated) **Deprecated.** Ignored. Use `settings.period` instead.


<a id="nestedblock--settings--ip"></a>
### Nested Schema for `settings.ip`

Required:

- `targets` (List of String) List of IP addresses of targets.

Optional:

- `use_local_ip` (Boolean) Boolean value indicating whether to use local (private) IP address of the target agents.


<a id="nestedblock--settings--monitoring_settings"></a>
### Nested Schema for `settings.monitoring_settings`

Optional:

- `notification_channels` (List of String, Deprecated) **Deprecated.** Use `settings.notification_channels` instead.


<a id="nestedblock--settings--network_grid"></a>
### Nested Schema for `settings.network_grid`

Required:

- `targets` (List of String) List of IP addresses of targets.

Optional:

- `use_local_ip` (Boolean) Boolean value indicating whether to use local (private) IP address of the target agents.


<a id="nestedblock--settings--network_mesh"></a>
### Nested Schema for `settings.network_mesh`

Optional:

- `use_local_ip` (Boolean) Boolean value indicating whether to use local (private) IP address of the target agents.


<a id="nestedblock--settings--page_load"></a>
### Nested Schema for `settings.page_load`

Required:

- `target` (String) HTTP or HTTPS URL to request.

Optional:

- `css_selectors` (Map of String) Map of CSS selector values keyed by selector name.
- `headers` (Map of String) Map of HTTP header values keyed by header names.
- `ignore_tls_errors` (Boolean) Boolean indicating whether to ignore TLS certificate verification errors.
- `timeout` (Number) HTTP transaction timeout (in milliseconds).


<a id="nestedblock--settings--ping"></a>
### Nested Schema for `settings.ping`

Optional:

- `count` (Number) Number of probe packets to send in one iteration.
- `delay` (Number) Inter-probe delay in milliseconds.
- `dscp` (Number) DSCP code to be set in IP header of probe packets.
- `expiry` (Number, Deprecated) **Deprecated.** Use `timeout` instead.
- `period` (Number, Deprecated) **Deprecated.** Ignored. Use `settings.period` instead.
- `port` (Number) Target port for TCP probes (ignored for ICMP).
- `protocol` (String) Transport protocol to use (icmp | tcp). Valid values: `icmp`, `tcp`.
- `timeout` (Number) Timeout in milliseconds for execution of the task.


<a id="nestedblock--settings--schedule"></a>
### Nested Schema for `settings.schedule`

Optional:

- `enabled` (Boolean) Boolean indicating an (dis|en)abled schedule.
- `end` (Number) UTC unix timestamp indicating the end of the schedule.
- `start` (Number) UTC unix timestamps indicating the start of the schedule.


<a id="nestedblock--settings--site"></a>
### Nested Schema for `settings.site`

Optional:

- `target` (String) Site ID.


<a id="nestedblock--settings--tag"></a>
### Nested Schema for `settings.tag`

Optional:

- `target` (String) Tag.


<a id="nestedblock--settings--throughput"></a>
### Nested Schema for `settings.throughput`

Optional:

- `bandwidth` (Number) (optional) Target bandwidth in Mbps, if no bandwidth is specified, the test will measure the maximum bandwidth for TCP and 10Mbps for UDP.
- `duration` (Number) Duration of the test in seconds.
- `omit` (Number) (optional) Number of seconds to omit from the start of the test.
- `port` (Number) Target port for TCP or UDP throughput task (the port the target server is listening on).
- `protocol` (String) Transport protocol to use (tcp | udp).


<a id="nestedblock--settings--trace"></a>
### Nested Schema for `settings.trace`

Optional:

- `count` (Number) Number of probe packets to send in one iteration.
- `delay` (Number) Inter-probe delay in milliseconds.
- `dscp` (Number) DSCP code to be set in IP header of probe packets.
- `expiry` (Number, Deprecated) **Deprecated.** Use `timeout` instead.
- `limit` (Number) Maximum number of hops to probe (i.e. maximum TTL).
- `mtu` (Boolean) Enable MTU in trace results.
- `period` (Number, Deprecated) **Deprecated.** Ignored. Use `settings.period` instead.
- `port` (Number) Target port for TCP or UDP probes (ignored for ICMP).
- `protocol` (String) Transport protocol to use (icmp | tcp | udp). Valid values: `icmp`, `tcp`, `udp`.
- `timeout` (Number) Timeout in milliseconds for execution of the task.


<a id="nestedblock--settings--url"></a>
### Nested Schema for `settings.url`

Required:

- `target` (String) HTTP or HTTPS URL to request.

Optional:

- `body` (String) HTTP request body.
- `headers` (Map of String) Map of HTTP header values keyed by header names.
- `ignore_tls_errors` (Boolean) Boolean indicating whether to ignore TLS certificate verification errors.
- `method` (String) HTTP method to use (GET | HEAD | PATCH | POST | PUT).
- `timeout` (Number) HTTP transaction timeout (in milliseconds).



<a id="nestedatt--created_by"></a>
### Nested Schema for `created_by`

Read-Only:

- `email` (String)
- `full_name` (String)
- `id` (String)


<a id="nestedatt--last_updated_by"></a>
### Nested Schema for `last_updated_by`

Read-Only:

- `email` (String)
- `full_name` (String)
- `id` (String)

## Import

Import is supported using the following syntax:

```shell
# Tests are imported by ID.
terraform import kentik-synthetics_test.hostname 12345
```
