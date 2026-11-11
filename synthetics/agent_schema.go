package synthetics

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	syntheticspb "github.com/kentik/api-schema-public/gen/go/kentik/synthetics/v202309"
)

// makeAgentSchema returns the agent schema. In create mode (the kentik-synthetics_agent resource) the
// attributes that UpdateAgent accepts are Optional+Computed; everything else is read-only.
func makeAgentSchema(mode schemaMode) map[string]*schema.Schema {
	b := newBuilder(mode, md(&syntheticspb.Agent{}))
	rw := optionalComputed
	s := map[string]*schema.Schema{
		idKey: {
			Type:        schema.TypeString,
			Required:    mode == readSingle,
			Computed:    mode != readSingle,
			Description: "ID of the agent.",
		},
		"site_name": b.field("site_name", schema.TypeString, rw),
		"status": b.field("status", schema.TypeString, rw,
			enumOf(syntheticspb.AgentStatus(0).Descriptor(), "AGENT_STATUS_DELETED"),
			description("Operational status of the agent. Set `AGENT_STATUS_OK` to authorize a private agent "+
				"that is waiting (`AGENT_STATUS_WAIT`).")),
		"alias":          b.field("alias", schema.TypeString, rw),
		"type":           b.field("type", schema.TypeString, computedOnly),
		"os":             b.field("os", schema.TypeString, computedOnly),
		"ip":             b.field("ip", schema.TypeString, computedOnly),
		"lat":            b.field("lat", schema.TypeFloat, rw),
		"long":           b.field("long", schema.TypeFloat, rw),
		"last_authed":    b.field("last_authed", schema.TypeString, computedOnly),
		"family":         b.field("family", schema.TypeString, rw, enumOf(syntheticspb.IPFamily(0).Descriptor())),
		"asn":            b.field("asn", schema.TypeInt, computedOnly),
		"site_id":        b.field("site_id", schema.TypeString, rw),
		"version":        b.field("version", schema.TypeString, computedOnly),
		"city":           b.field("city", schema.TypeString, rw),
		"region":         b.field("region", schema.TypeString, rw),
		"country":        b.field("country", schema.TypeString, rw),
		"test_ids":       b.field("test_ids", schema.TypeList, computedOnly, listOf(schema.TypeString)),
		"local_ip":       b.field("local_ip", schema.TypeString, computedOnly),
		"cloud_region":   b.field("cloud_region", schema.TypeString, computedOnly),
		"cloud_provider": b.field("cloud_provider", schema.TypeString, computedOnly),
		"agent_impl": b.field("agent_impl", schema.TypeString, computedOnly,
			description("Agent implementation: `IMPLEMENT_TYPE_RUST` (network agent), `IMPLEMENT_TYPE_NODE` (app "+
				"agent with Chromium) or `IMPLEMENT_TYPE_NETWORK` (app agent without Chromium).")),
		"labels": b.field("labels", schema.TypeSet, rw, setOf(schema.TypeString)),
		"metadata": b.block("metadata", computedOnly, func(b builder) map[string]*schema.Schema {
			ips := func(name string) *schema.Schema {
				return b.field(name, schema.TypeList, computedOnly, listOf(schema.TypeString))
			}
			return map[string]*schema.Schema{
				"private_ipv4_addresses": ips("private_ipv4_addresses"),
				"public_ipv4_addresses":  ips("public_ipv4_addresses"),
				"private_ipv6_addresses": ips("private_ipv6_addresses"),
				"public_ipv6_addresses":  ips("public_ipv6_addresses"),
				"static_ipv4":            b.field("static_ipv4", schema.TypeBool, computedOnly),
				"static_ipv6":            b.field("static_ipv6", schema.TypeBool, computedOnly),
			}
		}),
		// Deprecated v202101beta1 attributes.
		"name": finalize(mode, computedOnly, &schema.Schema{
			Type:        schema.TypeString,
			Description: "**Deprecated.** Use `site_name` instead (same value).",
			Deprecated:  "Use site_name instead.",
		}),
		"cloud_vpc": finalize(mode, computedOnly, &schema.Schema{
			Type:        schema.TypeString,
			Description: "**Deprecated.** Not provided by synthetics API v202309; always empty.",
			Deprecated:  "Not provided by synthetics API v202309; always empty.",
		}),
	}
	return s
}

func agentToTF(a *syntheticspb.Agent, s map[string]*schema.Schema) map[string]any {
	m := messageToTF(a.ProtoReflect(), s)
	m["name"] = a.GetSiteName()
	m["cloud_vpc"] = ""
	return m
}
