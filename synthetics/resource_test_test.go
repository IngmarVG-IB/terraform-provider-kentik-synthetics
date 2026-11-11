package synthetics_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	syntheticspb "github.com/kentik/api-schema-public/gen/go/kentik/synthetics/v202309"
	"github.com/stretchr/testify/require"

	"github.com/kentik/terraform-provider-kentik-synthetics/internal/fakesynthetics"
)

// Every test type has an example in examples/test-types/<type>/main.tf; the examples are applied verbatim.
var exampleTestTypes = map[string]func(*syntheticspb.Test) error{
	"hostname": func(t *syntheticspb.Test) error {
		return all(
			eq("hostname.target", t.GetSettings().GetHostname().GetTarget(), "www.example.com"),
			eq("ping.count", t.GetSettings().GetPing().GetCount(), uint32(5)),
			eq("trace.protocol", t.GetSettings().GetTrace().GetProtocol(), "udp"),
			eq("alerting_type", t.GetSettings().GetAlerting().GetAlertingType(), syntheticspb.AlertingType_ALERTING_TYPE_GROUPED),
			eq("grouped default percent", t.GetSettings().GetAlerting().GetGroupedAlertSettings().GetDefault().GetPercentOfSrcGroup(), uint32(50)),
			eq("activation.times", t.GetSettings().GetHealthSettings().GetActivation().GetTimes(), "3"),
			eq("labels", len(t.GetLabels()), 2),
			eq("family", t.GetSettings().GetFamily(), syntheticspb.IPFamily_IP_FAMILY_V4),
		)
	},
	"ip": func(t *syntheticspb.Test) error {
		return all(eq("ip.targets", len(t.GetSettings().GetIp().GetTargets()), 2),
			eq("ping.port", t.GetSettings().GetPing().GetPort(), uint32(443)))
	},
	"network_grid": func(t *syntheticspb.Test) error {
		return eq("network_grid.targets", len(t.GetSettings().GetNetworkGrid().GetTargets()), 3)
	},
	"agent": func(t *syntheticspb.Test) error {
		return all(eq("agent.target", t.GetSettings().GetAgent().GetTarget(), privateAgentID),
			eq("agent.reciprocal", t.GetSettings().GetAgent().GetReciprocal(), true),
			eq("throughput.port", t.GetSettings().GetThroughput().GetPort(), uint32(5201)))
	},
	"network_mesh": func(t *syntheticspb.Test) error {
		return all(eq("network_mesh set", t.GetSettings().GetNetworkMesh() != nil, true),
			eq("network_mesh.use_local_ip", t.GetSettings().GetNetworkMesh().GetUseLocalIp(), true))
	},
	"flow": func(t *syntheticspb.Test) error {
		return all(eq("flow.type", t.GetSettings().GetFlow().GetType(), "asn"),
			eq("flow.max_ip_targets", t.GetSettings().GetFlow().GetMaxIpTargets(), uint32(10)))
	},
	"url": func(t *syntheticspb.Test) error {
		return all(eq("url.method", t.GetSettings().GetUrl().GetMethod(), "GET"),
			eq("url.headers", t.GetSettings().GetUrl().GetHeaders()["Accept"], "application/json"),
			eq("status", t.GetStatus(), syntheticspb.TestStatus_TEST_STATUS_PAUSED),
			eq("http_valid_codes", len(t.GetSettings().GetHealthSettings().GetHttpValidCodes()), 2))
	},
	"page_load": func(t *syntheticspb.Test) error {
		return eq("css_selectors", t.GetSettings().GetPageLoad().GetCssSelectors()["logo"], "#logo")
	},
	"dns": func(t *syntheticspb.Test) error {
		return all(eq("dns.record_type", t.GetSettings().GetDns().GetRecordType(), syntheticspb.DNSRecord_DNS_RECORD_A),
			eq("dns.port", t.GetSettings().GetDns().GetPort(), uint32(53)))
	},
	"dns_grid": func(t *syntheticspb.Test) error {
		return all(eq("dns_grid.target", t.GetSettings().GetDnsGrid().GetTarget(), "www.example.com"),
			eq("dns_grid.servers", len(t.GetSettings().GetDnsGrid().GetServers()), 2))
	},
}

func TestResourceTest_ExamplesForEveryTestType(t *testing.T) {
	t.Parallel()
	dirs, err := filepath.Glob("../examples/test-types/*")
	require.NoError(t, err)
	var seen []string
	for _, dir := range dirs {
		typ := filepath.Base(dir)
		seen = append(seen, typ)
		check, ok := exampleTestTypes[typ]
		require.Truef(t, ok, "no assertions for example %s", typ)

		t.Run(typ, func(t *testing.T) {
			t.Parallel()
			fake := startFake(t)
			name := "kentik-synthetics_test." + typ
			resource.UnitTest(t, resource.TestCase{
				ProtoV5ProviderFactories: protoV5Factories(fake),
				CheckDestroy:             checkNoTests(fake),
				Steps: []resource.TestStep{{
					Config: providerConfig + readExample(t, filepath.Join(dir, "main.tf")),
					Check: resource.ComposeAggregateTestCheckFunc(
						resource.TestCheckResourceAttrSet(name, "id"),
						resource.TestCheckResourceAttr(name, "type", typ),
						resource.TestCheckResourceAttrSet(name, "cdate"),
						resource.TestCheckResourceAttr(name, "created_by.0.email", fakesynthetics.FakeUserEmail),
						resource.TestCheckResourceAttr(name, "settings.0.period", "60"),
						resource.TestCheckResourceAttr(name, "settings.0."+typ+".#", "1"),
						checkTestStored(fake, name, func(st *syntheticspb.Test) error {
							return all(eq("type", st.GetType(), typ), check(st))
						}),
					),
				}, {
					ResourceName:            name,
					ImportState:             true,
					ImportStateVerify:       true,
					ImportStateVerifyIgnore: []string{"status"}, // user spelling ("paused") is kept in state
				}},
			})
		})
	}
	for typ := range exampleTestTypes {
		require.Containsf(t, seen, typ, "missing examples/test-types/%s/main.tf", typ)
	}
}

func TestResourceTest_UpdateAndStatus(t *testing.T) {
	t.Parallel()
	fake := startFake(t)
	const name = "kentik-synthetics_test.t"
	cfg := func(target, status string, count int) string {
		return providerConfig + fmt.Sprintf(`
resource "kentik-synthetics_test" "t" {
  name   = "lifecycle-%[1]s"
  type   = "hostname"
  status = %[2]q
  labels = ["a"]
  settings {
    hostname { target = %[1]q }
    agent_ids = ["968"]
    tasks     = ["ping", "traceroute"]
    ping { count = %[3]d }
  }
}`, target, status, count)
	}
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: protoV5Factories(fake),
		CheckDestroy:             checkNoTests(fake),
		Steps: []resource.TestStep{{
			Config: cfg("a.example.com", "TEST_STATUS_ACTIVE", 3),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr(name, "status", "TEST_STATUS_ACTIVE"),
				resource.TestCheckResourceAttr(name, "settings.0.ping.0.count", "3"),
				expectCalls(fake, "SetTestStatus", 0),
			),
		}, {
			// Configuration and status change: UpdateTest, then SetTestStatus.
			Config: cfg("b.example.com", "TEST_STATUS_PAUSED", 4),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr(name, "name", "lifecycle-b.example.com"),
				resource.TestCheckResourceAttr(name, "status", "TEST_STATUS_PAUSED"),
				resource.TestCheckResourceAttr(name, "settings.0.ping.0.count", "4"),
				expectCalls(fake, "UpdateTest", 1),
				expectCalls(fake, "SetTestStatus", 1),
				checkTestStored(fake, name, func(st *syntheticspb.Test) error {
					return all(eq("status", st.GetStatus(), syntheticspb.TestStatus_TEST_STATUS_PAUSED),
						eq("target", st.GetSettings().GetHostname().GetTarget(), "b.example.com"))
				}),
			),
		}, {
			// Status only (alias spelling): SetTestStatus without UpdateTest.
			Config: cfg("b.example.com", "active", 4),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr(name, "status", "active"),
				expectCalls(fake, "UpdateTest", 1),
				expectCalls(fake, "SetTestStatus", 2),
				checkTestStored(fake, name, func(st *syntheticspb.Test) error {
					return eq("status", st.GetStatus(), syntheticspb.TestStatus_TEST_STATUS_ACTIVE)
				}),
			),
		}, {
			// Same status, different spelling: no change.
			Config:   cfg("b.example.com", "TEST_STATUS_ACTIVE", 4),
			PlanOnly: true,
		}},
	})
}

func TestResourceTest_LegacyV02Config(t *testing.T) {
	t.Parallel()
	fake := startFake(t)
	stored := func(name string, check func(*syntheticspb.Test) error) resource.TestCheckFunc {
		return checkTestStored(fake, "kentik-synthetics_test."+name, check)
	}
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: protoV5Factories(fake),
		CheckDestroy:             checkNoTests(fake),
		Steps: []resource.TestStep{{
			// Applies cleanly, and the post-apply plan is empty (checked by the framework).
			Config: providerConfig + readExample(t, "testdata/legacy_v0_2.tf"),
			Check: resource.ComposeAggregateTestCheckFunc(
				stored("example-hostname-test", func(st *syntheticspb.Test) error {
					p, tr := st.GetSettings().GetPing(), st.GetSettings().GetTrace()
					return all(
						eq("ping.protocol <- settings.protocol", p.GetProtocol(), "tcp"),
						eq("ping.port <- settings.port", p.GetPort(), uint32(443)),
						eq("ping.timeout <- ping.expiry", p.GetTimeout(), uint32(3000)),
						eq("ping.count", p.GetCount(), uint32(5)),
						eq("trace.timeout <- trace.expiry", tr.GetTimeout(), uint32(22500)),
						eq("trace.limit", tr.GetLimit(), uint32(30)),
						eq("http_valid_codes", len(st.GetSettings().GetHealthSettings().GetHttpValidCodes()), 2),
					)
				}),
				stored("minimal-dns-test", func(st *syntheticspb.Test) error {
					d := st.GetSettings().GetDns()
					return all(eq("dns.record_type <- dns.type", d.GetRecordType(), syntheticspb.DNSRecord_DNS_RECORD_CNAME),
						eq("dns.servers <- settings.servers", len(d.GetServers()), 2))
				}),
				stored("minimal-dns-grid-test", func(st *syntheticspb.Test) error {
					d := st.GetSettings().GetDnsGrid()
					return all(eq("dns_grid.target <- targets[0]", d.GetTarget(), "www.example.com"),
						eq("dns_grid.servers", len(d.GetServers()), 2))
				}),
				stored("minimal-flow-test", func(st *syntheticspb.Test) error {
					return eq("flow.direction", st.GetSettings().GetFlow().GetDirection(), "src")
				}),
				resource.TestCheckResourceAttr("kentik-synthetics_test.example-hostname-test", "settings.0.rollup_level", "1"),
				resource.TestCheckResourceAttr("kentik-synthetics_test.example-hostname-test", "settings.0.ping.0.expiry", "3000"),
			),
		}},
	})
}

func TestResourceTest_Validation(t *testing.T) {
	t.Parallel()
	fake := startFake(t)
	base := func(typ, blocks, extra string) string {
		return providerConfig + fmt.Sprintf(`
resource "kentik-synthetics_test" "bad" {
  name = "bad"
  type = %q
  %s
  settings {
    %s
    agent_ids = ["968"]
    tasks     = ["ping"]
  }
}`, typ, extra, blocks)
	}
	cases := map[string]struct{ cfg, err string }{
		"type mismatch":    {base("ip", `hostname { target = "x" }`, ""), `does not match the settings.hostname block`},
		"two definitions":  {base("ip", `hostname { target = "x" }`+"\n"+`ip { targets = ["192.0.2.1"] }`, ""), `only one test definition block`},
		"missing block":    {base("url", "", ""), `requires the settings.url block`},
		"unsupported type": {base("site", `hostname { target = "x" }`, ""), `expected type to be one of`},
		"invalid status":   {base("hostname", `hostname { target = "x" }`, `status = "TEST_STATUS_DELETED"`), `invalid test status`},
		"invalid enum": {
			base("dns", `dns {
      target      = "x"
      record_type = "DNS_RECORD_TXT"
    }`, ""), `expected settings.0.dns.0.record_type to be one of`,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			resource.UnitTest(t, resource.TestCase{
				ProtoV5ProviderFactories: protoV5Factories(fake),
				Steps: []resource.TestStep{{
					Config: tc.cfg, PlanOnly: true, ExpectError: regexp.MustCompile(tc.err),
				}},
			})
		})
	}
}

func TestResourceTest_DeletedOutsideTerraform(t *testing.T) {
	t.Parallel()
	fake := startFake(t)
	cfg := providerConfig + readExample(t, "../examples/test-types/ip/main.tf")
	var id string
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: protoV5Factories(fake),
		Steps: []resource.TestStep{{
			Config: cfg,
			Check: func(s *terraform.State) error {
				id = s.RootModule().Resources["kentik-synthetics_test.ip"].Primary.ID
				return nil
			},
		}, {
			// The test disappears from the API: it is dropped from state and planned for re-creation.
			PreConfig:          func() { fake.RemoveTest(id) },
			Config:             cfg,
			PlanOnly:           true,
			ExpectNonEmptyPlan: true,
		}},
	})
}

func readExample(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(b)
}

func checkNoTests(fake *fakesynthetics.Server) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		for _, rs := range s.RootModule().Resources {
			if rs.Type == "kentik-synthetics_test" && fake.Test(rs.Primary.ID) != nil {
				return fmt.Errorf("test %s still exists", rs.Primary.ID)
			}
		}
		return nil
	}
}

func expectCalls(fake *fakesynthetics.Server, method string, n int) resource.TestCheckFunc {
	return func(*terraform.State) error {
		if got := fake.CallCount(method); got != n {
			return fmt.Errorf("%s called %d times, want %d", method, got, n)
		}
		return nil
	}
}

func eq[T comparable](what string, got, want T) error {
	if got != want {
		return fmt.Errorf("%s: got %v, want %v", what, got, want)
	}
	return nil
}

func all(errs ...error) error {
	return errors.Join(slices.DeleteFunc(errs, func(e error) bool { return e == nil })...)
}
