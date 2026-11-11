package synthetics_test

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	syntheticspb "github.com/kentik/api-schema-public/gen/go/kentik/synthetics/v202309"
)

func TestDataSourceAgent(t *testing.T) {
	t.Parallel()
	fake := startFake(t)
	const ds = "data.kentik-synthetics_agent.a"
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: protoV5Factories(fake),
		Steps: []resource.TestStep{{
			Config: providerConfig + `data "kentik-synthetics_agent" "a" { id = "968" }`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr(ds, "id", globalAgentID),
				resource.TestCheckResourceAttr(ds, "site_name", "global-agent"),
				resource.TestCheckResourceAttr(ds, "name", "global-agent"), // deprecated alias
				resource.TestCheckResourceAttr(ds, "status", "AGENT_STATUS_OK"),
				resource.TestCheckResourceAttr(ds, "alias", "probe-4-ams-1"),
				resource.TestCheckResourceAttr(ds, "type", "global"),
				resource.TestCheckResourceAttr(ds, "lat", "52.374031"),
				resource.TestCheckResourceAttr(ds, "long", "4.88969"),
				resource.TestCheckResourceAttr(ds, "last_authed", "2026-10-01T12:00:00Z"),
				resource.TestCheckResourceAttr(ds, "family", "IP_FAMILY_DUAL"),
				resource.TestCheckResourceAttr(ds, "asn", "20473"),
				resource.TestCheckResourceAttr(ds, "city", "Amsterdam"),
				resource.TestCheckResourceAttr(ds, "test_ids.#", "2"),
				resource.TestCheckResourceAttr(ds, "agent_impl", "IMPLEMENT_TYPE_RUST"),
				resource.TestCheckResourceAttr(ds, "labels.#", "2"),
				resource.TestCheckResourceAttr(ds, "metadata.0.public_ipv4_addresses.0", "192.0.2.10"),
				resource.TestCheckResourceAttr(ds, "metadata.0.static_ipv4", "true"),
				resource.TestCheckResourceAttr(ds, "cloud_vpc", ""),
			),
		}, {
			Config:      providerConfig + `data "kentik-synthetics_agent" "a" { id = "404" }`,
			ExpectError: regexp.MustCompile(`NotFound`),
		}},
	})
}

func TestDataSourceAgents(t *testing.T) {
	t.Parallel()
	fake := startFake(t)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: protoV5Factories(fake),
		Steps: []resource.TestStep{{
			Config: providerConfig + `
data "kentik-synthetics_agents" "all" {}
data "kentik-synthetics_agents" "near_amsterdam" {
  latitude     = 52.37
  longitude    = 4.89
  max_distance = 1000
}
data "kentik-synthetics_agents" "far_from_amsterdam" {
  latitude     = 52.37
  longitude    = 4.89
  min_distance = 1000
}`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("data.kentik-synthetics_agents.all", "items.#", "2"),
				resource.TestCheckResourceAttr("data.kentik-synthetics_agents.all", "invalid_agents_count", "0"),
				resource.TestCheckResourceAttr("data.kentik-synthetics_agents.all", "items.1.id", privateAgentID),
				resource.TestCheckResourceAttr("data.kentik-synthetics_agents.all", "items.1.status", "AGENT_STATUS_WAIT"),
				resource.TestCheckResourceAttr("data.kentik-synthetics_agents.near_amsterdam", "items.#", "1"),
				resource.TestCheckResourceAttr("data.kentik-synthetics_agents.near_amsterdam", "items.0.id", globalAgentID),
				resource.TestCheckResourceAttr("data.kentik-synthetics_agents.far_from_amsterdam", "items.#", "1"),
				resource.TestCheckResourceAttr("data.kentik-synthetics_agents.far_from_amsterdam", "items.0.id", privateAgentID),
			),
		}},
	})
}

func seedTest(fake interface {
	AddTest(*syntheticspb.Test) string
},
) string {
	return fake.AddTest(&syntheticspb.Test{
		Name: "seeded dns test", Type: "dns", Status: syntheticspb.TestStatus_TEST_STATUS_PAUSED,
		Labels: []string{"seed"},
		Settings: &syntheticspb.TestSettings{
			Definition: &syntheticspb.TestSettings_Dns{Dns: &syntheticspb.DnsTest{
				Target: "example.com", RecordType: syntheticspb.DNSRecord_DNS_RECORD_MX, Servers: []string{"192.0.2.53"},
			}},
			AgentIds:             []string{globalAgentID},
			Tasks:                []string{"dns"},
			Period:               300,
			NotificationChannels: []string{"nc-1"},
			Ping:                 &syntheticspb.TestPingSettings{Timeout: 1500, Protocol: "icmp"},
		},
	})
}

func TestDataSourceTest(t *testing.T) {
	t.Parallel()
	fake := startFake(t)
	id := seedTest(fake)
	const ds = "data.kentik-synthetics_test.t"
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: protoV5Factories(fake),
		Steps: []resource.TestStep{{
			Config: providerConfig + `data "kentik-synthetics_test" "t" { id = "` + id + `" }`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr(ds, "name", "seeded dns test"),
				resource.TestCheckResourceAttr(ds, "type", "dns"),
				resource.TestCheckResourceAttr(ds, "status", "TEST_STATUS_PAUSED"),
				resource.TestCheckResourceAttr(ds, "labels.#", "1"),
				resource.TestCheckResourceAttr(ds, "settings.0.dns.0.target", "example.com"),
				resource.TestCheckResourceAttr(ds, "settings.0.dns.0.record_type", "DNS_RECORD_MX"),
				resource.TestCheckResourceAttr(ds, "settings.0.period", "300"),
				resource.TestCheckResourceAttr(ds, "settings.0.hostname.#", "0"),
				// Deprecated attributes are filled from their replacements.
				resource.TestCheckResourceAttr(ds, "settings.0.dns.0.type", "DNS_RECORD_MX"),
				resource.TestCheckResourceAttr(ds, "settings.0.servers.0", "192.0.2.53"),
				resource.TestCheckResourceAttr(ds, "settings.0.monitoring_settings.0.notification_channels.0", "nc-1"),
				resource.TestCheckResourceAttr(ds, "settings.0.ping.0.expiry", "1500"),
				resource.TestCheckResourceAttr(ds, "settings.0.protocol", "icmp"),
				resource.TestCheckResourceAttr(ds, "expires_on", ""),
			),
		}},
	})
}

func TestDataSourceTests(t *testing.T) {
	t.Parallel()
	fake := startFake(t)
	seedTest(fake)
	seedTest(fake)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: protoV5Factories(fake),
		Steps: []resource.TestStep{{
			Config: providerConfig + `data "kentik-synthetics_tests" "all" {}`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("data.kentik-synthetics_tests.all", "items.#", "2"),
				resource.TestCheckResourceAttr("data.kentik-synthetics_tests.all", "items.1.settings.0.dns.0.servers.0", "192.0.2.53"),
				resource.TestCheckResourceAttr("data.kentik-synthetics_tests.all", "invalid_tests_count", "0"),
			),
		}},
	})
}
