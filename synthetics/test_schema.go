package synthetics

import (
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	syntheticspb "github.com/kentik/api-schema-public/gen/go/kentik/synthetics/v202309"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// testSchemaVersion is the state schema version of kentik-synthetics_test.
// Version 0: provider <= 0.2.x (synthetics v202101beta1). Version 1: synthetics v202309.
const testSchemaVersion = 1

// testTypes are the test types supported by synthetics v202309, in the order of the API documentation.
var testTypes = []string{ //nolint:gochecknoglobals // constant list
	"hostname", "ip", "network_grid", "agent", "network_mesh", "flow", "url", "page_load", "dns", "dns_grid",
}

// definitionBlocks are the settings blocks of the TestSettings.definition oneof; exactly one is set, named
// after the test type.
var definitionBlocks = testTypes //nolint:gochecknoglobals // constant list

const (
	statusActive = "TEST_STATUS_ACTIVE"
	statusPaused = "TEST_STATUS_PAUSED"
)

func md(m interface {
	ProtoReflect() protoreflect.Message
},
) protoreflect.MessageDescriptor {
	return m.ProtoReflect().Descriptor()
}

func makeTestSchema(mode schemaMode) map[string]*schema.Schema {
	b := newBuilder(mode, md(&syntheticspb.Test{}))
	s := map[string]*schema.Schema{
		idKey: {
			Type:        schema.TypeString,
			Required:    mode == readSingle,
			Computed:    mode != readSingle,
			Description: "ID of the test.",
		},
		"name": b.field("name", schema.TypeString, required),
		"type": b.field("type", schema.TypeString, required, forceNew(), oneOf(testTypes...),
			description("Type of the test. Changing it forces a new test; the matching `settings` block "+
				"(for example `settings.hostname` for `hostname`) must be set.")),
		"status": b.field("status", schema.TypeString, optional, withDefault(statusActive),
			validate(validateTestStatus), func(s *schema.Schema) {
				s.DiffSuppressFunc = func(_, oldV, newV string, _ *schema.ResourceData) bool {
					return normalizeTestStatus(oldV) == normalizeTestStatus(newV)
				}
			},
			description("Operational status of the test: `TEST_STATUS_ACTIVE` (default) or `TEST_STATUS_PAUSED`; "+
				"`active` and `paused` are accepted too. Changes are applied with the SetTestStatus API call, "+
				"without re-creating the test.")),
		"labels":          b.field("labels", schema.TypeSet, optional, setOf(schema.TypeString)),
		"settings":        b.block("settings", required, testSettingsFields),
		"cdate":           b.field("cdate", schema.TypeString, computedOnly),
		"edate":           b.field("edate", schema.TypeString, computedOnly),
		"created_by":      b.block("created_by", computedOnly, userInfoFields),
		"last_updated_by": b.block("last_updated_by", computedOnly, userInfoFields),
		"expires_on": finalize(mode, computedOnly, &schema.Schema{
			Type:        schema.TypeString,
			Description: "**Deprecated.** Not provided by synthetics API v202309; always empty.",
			Deprecated:  "Not provided by synthetics API v202309; always empty.",
		}),
	}
	return s
}

func userInfoFields(b builder) map[string]*schema.Schema {
	return map[string]*schema.Schema{
		idKey:       b.field("id", schema.TypeString, computedOnly),
		"email":     b.field("email", schema.TypeString, computedOnly),
		"full_name": b.field("full_name", schema.TypeString, computedOnly),
	}
}

func testSettingsFields(b builder) map[string]*schema.Schema {
	ipFamily := syntheticspb.IPFamily(0).Descriptor()
	s := map[string]*schema.Schema{
		// Type-specific definitions (TestSettings.definition oneof).
		"hostname": b.block("hostname", optional, func(b builder) map[string]*schema.Schema {
			return map[string]*schema.Schema{"target": b.field("target", schema.TypeString, required)}
		}),
		"ip":           b.block("ip", optional, ipTestFields),
		"network_grid": b.block("network_grid", optional, ipTestFields),
		"agent": b.block("agent", optional, func(b builder) map[string]*schema.Schema {
			return map[string]*schema.Schema{
				"target":       b.field("target", schema.TypeString, required),
				"use_local_ip": b.field("use_local_ip", schema.TypeBool, optionalComputed),
				"reciprocal":   b.field("reciprocal", schema.TypeBool, optionalComputed),
			}
		}),
		"network_mesh": b.block("network_mesh", optional, func(b builder) map[string]*schema.Schema {
			return map[string]*schema.Schema{
				"use_local_ip": b.field("use_local_ip", schema.TypeBool, optionalComputed),
			}
		}),
		"flow":      b.block("flow", optional, flowTestFields),
		"url":       b.block("url", optional, urlTestFields),
		"page_load": b.block("page_load", optional, pageLoadTestFields),
		"dns":       b.block("dns", optional, func(b builder) map[string]*schema.Schema { return dnsTestFields(b, false) }),
		"dns_grid":  b.block("dns_grid", optional, func(b builder) map[string]*schema.Schema { return dnsTestFields(b, true) }),

		// Common settings.
		"agent_ids": b.field("agent_ids", schema.TypeList, required, listOf(schema.TypeString)),
		"tasks": b.field("tasks", schema.TypeList, required, listOf(schema.TypeString),
			description("List of task names to run for the test: `ping`, `traceroute`, `http`, `page-load`, "+
				"`dns`, `throughput`. Which tasks are valid depends on the test type.")),
		"period": b.field("period", schema.TypeInt, optionalComputed),
		"family": b.field("family", schema.TypeString, optionalComputed, enumOf(ipFamily)),
		"notification_channels": b.field("notification_channels", schema.TypeList, optionalComputed,
			listOf(schema.TypeString)),
		"notes":           b.field("notes", schema.TypeString, optional),
		"health_settings": b.block("health_settings", optionalComputed, healthSettingsFields),
		"ping":            b.block("ping", optionalComputed, pingSettingsFields),
		"trace":           b.block("trace", optionalComputed, traceSettingsFields),
		"throughput": b.block("throughput", optionalComputed, func(b builder) map[string]*schema.Schema {
			return map[string]*schema.Schema{
				"port":      b.field("port", schema.TypeInt, optionalComputed),
				"omit":      b.field("omit", schema.TypeInt, optionalComputed),
				"duration":  b.field("duration", schema.TypeInt, optionalComputed),
				"bandwidth": b.field("bandwidth", schema.TypeInt, optionalComputed),
				"protocol":  b.field("protocol", schema.TypeString, optionalComputed),
			}
		}),
		"schedule": b.block("schedule", optionalComputed, func(b builder) map[string]*schema.Schema {
			return map[string]*schema.Schema{
				"enabled": b.field("enabled", schema.TypeBool, optionalComputed),
				"start":   b.field("start", schema.TypeInt, optionalComputed),
				"end":     b.field("end", schema.TypeInt, optionalComputed),
			}
		}),
		"alerting": b.block("alerting", optionalComputed, alertingSettingsFields),
	}

	for _, blk := range definitionBlocks {
		if s[blk].Description == "" {
			s[blk].Description = "Target definition of a `" + blk + "` test. Required when `type = \"" + blk + "\"`" +
				" and not allowed otherwise."
		}
	}
	s["network_mesh"].Description = "Target definition of a `network_mesh` test (all agents in `agent_ids` test each " +
		"other). Optional when `type = \"network_mesh\"` and not allowed otherwise."

	// Deprecated v202101beta1 attributes. Kept so existing configurations still parse; see docs/UPGRADING.md.
	for k, v := range legacySettingsFields(b) {
		s[k] = v
	}
	return s
}

func ipTestFields(b builder) map[string]*schema.Schema {
	return map[string]*schema.Schema{
		"targets":      b.field("targets", schema.TypeList, required, listOf(schema.TypeString)),
		"use_local_ip": b.field("use_local_ip", schema.TypeBool, optionalComputed),
	}
}

func flowTestFields(b builder) map[string]*schema.Schema {
	return map[string]*schema.Schema{
		"target":                         b.field("target", schema.TypeString, required),
		"target_refresh_interval_millis": b.field("target_refresh_interval_millis", schema.TypeInt, optionalComputed),
		"max_providers":                  b.field("max_providers", schema.TypeInt, optionalComputed),
		"max_ip_targets":                 b.field("max_ip_targets", schema.TypeInt, optionalComputed),
		"type": b.field("type", schema.TypeString, required,
			oneOf("asn", "cdn", "country", "region", "city")),
		"inet_direction": b.field("inet_direction", schema.TypeString, optionalComputed),
		"direction":      b.field("direction", schema.TypeString, optionalComputed),
		"max_tasks":      b.legacy("max_tasks", schema.TypeInt, "Use `max_ip_targets` instead."),
	}
}

func urlTestFields(b builder) map[string]*schema.Schema {
	return map[string]*schema.Schema{
		"target":            b.field("target", schema.TypeString, required),
		"timeout":           b.field("timeout", schema.TypeInt, optionalComputed),
		"method":            b.field("method", schema.TypeString, optionalComputed),
		"headers":           b.field("headers", schema.TypeMap, optionalComputed, mapOfString()),
		"body":              b.field("body", schema.TypeString, optionalComputed),
		"ignore_tls_errors": b.field("ignore_tls_errors", schema.TypeBool, optionalComputed),
	}
}

func pageLoadTestFields(b builder) map[string]*schema.Schema {
	return map[string]*schema.Schema{
		"target":            b.field("target", schema.TypeString, required),
		"timeout":           b.field("timeout", schema.TypeInt, optionalComputed),
		"headers":           b.field("headers", schema.TypeMap, optionalComputed, mapOfString()),
		"ignore_tls_errors": b.field("ignore_tls_errors", schema.TypeBool, optionalComputed),
		"css_selectors":     b.field("css_selectors", schema.TypeMap, optionalComputed, mapOfString()),
	}
}

func dnsTestFields(b builder, grid bool) map[string]*schema.Schema {
	targetKind := required
	if grid {
		// v202101beta1 dns_grid used a `targets` list; `target` is not required so old configurations parse.
		targetKind = optionalComputed
	}
	s := map[string]*schema.Schema{
		"target":      b.field("target", schema.TypeString, targetKind),
		"record_type": b.field("record_type", schema.TypeString, optionalComputed, enumOf(syntheticspb.DNSRecord(0).Descriptor())),
		"servers":     b.field("servers", schema.TypeList, optionalComputed, listOf(schema.TypeString)),
		"port":        b.field("port", schema.TypeInt, optionalComputed),
		"timeout": b.field("timeout", schema.TypeInt, optionalComputed,
			deprecated("Ignored by the Kentik API."), description("**Deprecated.** Ignored by the Kentik API.")),
		"type": b.legacy("type", schema.TypeString, "Use `record_type` instead."),
	}
	if grid {
		s["targets"] = b.legacy("targets", schema.TypeList,
			"Use `target` (a single DNS name) and `servers` instead. Only the first element is used.",
			listOf(schema.TypeString))
	}
	return s
}

func healthSettingsFields(b builder) map[string]*schema.Schema {
	f := func(name string) *schema.Schema { return b.field(name, schema.TypeFloat, optionalComputed) }
	i := func(name string) *schema.Schema { return b.field(name, schema.TypeInt, optionalComputed) }
	t := func(name string) *schema.Schema { return b.field(name, schema.TypeBool, optionalComputed) }
	s := map[string]*schema.Schema{
		"http_valid_codes": b.field("http_valid_codes", schema.TypeList, optionalComputed, listOf(schema.TypeInt)),
		"dns_valid_codes":  b.field("dns_valid_codes", schema.TypeList, optionalComputed, listOf(schema.TypeInt)),
		"dns_valid_ips":    b.field("dns_valid_ips", schema.TypeString, optionalComputed),
		"activation": b.block("activation", optionalComputed, func(b builder) map[string]*schema.Schema {
			return map[string]*schema.Schema{
				"grace_period": b.field("grace_period", schema.TypeString, optionalComputed),
				"time_unit":    b.field("time_unit", schema.TypeString, optionalComputed),
				"time_window":  b.field("time_window", schema.TypeString, optionalComputed),
				"times":        b.field("times", schema.TypeString, optionalComputed),
			}
		}),
		"disabled_metrics": b.block("disabled_metrics", optionalComputed, func(b builder) map[string]*schema.Schema {
			out := map[string]*schema.Schema{}
			fields := b.msg.Fields()
			for idx := range fields.Len() {
				name := string(fields.Get(idx).Name())
				out[name] = b.field(name, schema.TypeBool, optionalComputed,
					description("Skip `"+name+"` during health evaluation."))
			}
			return out
		}),
		"cert_expiry_warning":  i("cert_expiry_warning"),
		"cert_expiry_critical": i("cert_expiry_critical"),
		"health_disabled":      t("health_disabled"),
		"disable_alerts":       t("disable_alerts"),
	}
	for _, name := range []string{
		"latency_critical", "latency_warning", "packet_loss_critical", "packet_loss_warning",
		"jitter_critical", "jitter_warning", "http_latency_critical", "http_latency_warning",
		"latency_critical_stddev", "latency_warning_stddev", "jitter_critical_stddev", "jitter_warning_stddev",
		"http_latency_critical_stddev", "http_latency_warning_stddev",
		"dns_latency_critical", "dns_latency_warning", "dns_latency_critical_stddev", "dns_latency_warning_stddev",
		"throughput_critical", "throughput_warning", "throughput_critical_stddev", "throughput_warning_stddev",
	} {
		s[name] = f(name)
	}
	return s
}

func pingSettingsFields(b builder) map[string]*schema.Schema {
	return map[string]*schema.Schema{
		"count":    b.field("count", schema.TypeInt, optionalComputed),
		"protocol": b.field("protocol", schema.TypeString, optionalComputed, oneOf("icmp", "tcp")),
		"port":     b.field("port", schema.TypeInt, optionalComputed),
		"timeout":  b.field("timeout", schema.TypeInt, optionalComputed),
		"delay":    b.field("delay", schema.TypeFloat, optionalComputed),
		"dscp":     b.field("dscp", schema.TypeInt, optionalComputed),
		"period":   b.legacy("period", schema.TypeFloat, "Ignored. Use `settings.period` instead."),
		"expiry":   b.legacy("expiry", schema.TypeFloat, "Use `timeout` instead."),
	}
}

func traceSettingsFields(b builder) map[string]*schema.Schema {
	return map[string]*schema.Schema{
		"count":    b.field("count", schema.TypeInt, optionalComputed),
		"protocol": b.field("protocol", schema.TypeString, optionalComputed, oneOf("icmp", "tcp", "udp")),
		"port":     b.field("port", schema.TypeInt, optionalComputed),
		"timeout":  b.field("timeout", schema.TypeInt, optionalComputed),
		"limit":    b.field("limit", schema.TypeInt, optionalComputed),
		"delay":    b.field("delay", schema.TypeFloat, optionalComputed),
		"dscp":     b.field("dscp", schema.TypeInt, optionalComputed),
		"mtu":      b.field("mtu", schema.TypeBool, optionalComputed),
		"period":   b.legacy("period", schema.TypeFloat, "Ignored. Use `settings.period` instead."),
		"expiry":   b.legacy("expiry", schema.TypeFloat, "Use `timeout` instead."),
	}
}

func alertingSettingsFields(b builder) map[string]*schema.Schema {
	grouped := func(b builder) map[string]*schema.Schema {
		return map[string]*schema.Schema{
			"metric":               b.field("metric", schema.TypeString, optionalComputed),
			"src_group_by":         b.field("src_group_by", schema.TypeString, optionalComputed, enumOf(syntheticspb.SrcGroupBy(0).Descriptor())),
			"percent_of_src_group": b.field("percent_of_src_group", schema.TypeInt, optionalComputed),
			"filter_ids":           b.field("filter_ids", schema.TypeList, optional, listOf(schema.TypeInt)),
		}
	}
	return map[string]*schema.Schema{
		"disable_warning_notifications": b.field("disable_warning_notifications", schema.TypeBool, optionalComputed),
		"alerting_type": b.field("alerting_type", schema.TypeString, optionalComputed,
			enumOf(syntheticspb.AlertingType(0).Descriptor())),
		"grouped_alert_settings": b.block("grouped_alert_settings", optionalComputed, func(b builder) map[string]*schema.Schema {
			return map[string]*schema.Schema{
				"default":   b.block("default", optionalComputed, grouped),
				"overrides": b.blockList("overrides", optional, grouped),
			}
		}),
	}
}

// legacySettingsFields are v202101beta1 settings without a v202309 field.
func legacySettingsFields(b builder) map[string]*schema.Schema {
	target := func(desc string) map[string]*schema.Schema {
		return map[string]*schema.Schema{"target": {Type: schema.TypeString, Optional: b.mode == create, Computed: b.mode != create, Description: desc}}
	}
	return map[string]*schema.Schema{
		"site": b.legacyBlock("`site` tests are not supported by synthetics API v202309 and are ignored.",
			target("Site ID.")),
		"tag": b.legacyBlock("`tag` tests are not supported by synthetics API v202309 and are ignored.",
			target("Tag.")),
		"count":        b.legacy("count", schema.TypeInt, "Ignored. Use `ping.count` / `trace.count` instead."),
		"expiry":       b.legacy("expiry", schema.TypeInt, "Ignored. Use `ping.timeout` / `trace.timeout` instead."),
		"limit":        b.legacy("limit", schema.TypeInt, "Ignored. Use `trace.limit` instead."),
		"port":         b.legacy("port", schema.TypeInt, "Use `ping.port` instead."),
		"protocol":     b.legacy("protocol", schema.TypeString, "Use `ping.protocol` instead."),
		"servers":      b.legacy("servers", schema.TypeList, "Use `dns.servers` / `dns_grid.servers` instead.", listOf(schema.TypeString)),
		"rollup_level": b.legacy("rollup_level", schema.TypeInt, "Ignored by synthetics API v202309."),
		"monitoring_settings": b.legacyBlock("Use `settings.notification_channels` instead.", map[string]*schema.Schema{
			"notification_channels": legacyAttr(b.mode, schema.TypeList, "Use `settings.notification_channels` instead.", listOf(schema.TypeString)),
		}),
		"http": b.legacyBlock("Use the `url` or `page_load` block instead.", map[string]*schema.Schema{
			"period":            legacyAttr(b.mode, schema.TypeInt, "Ignored. Use `settings.period` instead."),
			"expiry":            legacyAttr(b.mode, schema.TypeInt, "Use `url.timeout` / `page_load.timeout` instead."),
			"method":            legacyAttr(b.mode, schema.TypeString, "Use `url.method` instead."),
			"headers":           legacyAttr(b.mode, schema.TypeMap, "Use `url.headers` / `page_load.headers` instead.", mapOfString()),
			"body":              legacyAttr(b.mode, schema.TypeString, "Use `url.body` instead."),
			"ignore_tls_errors": legacyAttr(b.mode, schema.TypeBool, "Use `url.ignore_tls_errors` / `page_load.ignore_tls_errors` instead."),
			"css_selectors":     legacyAttr(b.mode, schema.TypeMap, "Use `page_load.css_selectors` instead.", mapOfString()),
		}),
	}
}

func legacyAttr(mode schemaMode, typ schema.ValueType, msg string, opts ...schemaOpt) *schema.Schema {
	return builder{mode: mode}.legacy("", typ, msg, opts...)
}

func normalizeTestStatus(s string) string {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "ACTIVE", statusActive, "":
		return statusActive
	case "PAUSED", statusPaused:
		return statusPaused
	default:
		return strings.ToUpper(s)
	}
}

func validateTestStatus(v any, k string) ([]string, []error) {
	s, _ := v.(string)
	switch normalizeTestStatus(s) {
	case statusActive, statusPaused:
		return nil, nil
	default:
		return nil, []error{errInvalidStatus(k, s)}
	}
}
