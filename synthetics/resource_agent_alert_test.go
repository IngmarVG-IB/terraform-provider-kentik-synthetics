package synthetics_test

import (
	"fmt"
	"slices"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	syntheticspb "github.com/kentik/api-schema-public/gen/go/kentik/synthetics/v202309"
)

func TestResourceAgentAlert(t *testing.T) {
	t.Parallel()
	fake := startFake(t)
	const name = "kentik-synthetics_agent_alert.offline"
	var id string
	cfg := func(agent string, threshold int, channels string) string {
		return providerConfig + fmt.Sprintf(`
resource "kentik-synthetics_agent_alert" "offline" {
  agent_id                 = %q
  threshold_seconds        = %d
  notification_channel_ids = [%s]
}`, agent, threshold, channels)
	}
	stored := func(check func(*syntheticspb.AgentAlert) error) resource.TestCheckFunc {
		return func(s *terraform.State) error {
			id = s.RootModule().Resources[name].Primary.ID
			a := fake.AgentAlert(id)
			if a == nil {
				return fmt.Errorf("alert %s not in fake", id)
			}
			return check(a)
		}
	}
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: protoV5Factories(fake),
		CheckDestroy: func(*terraform.State) error {
			if fake.AgentAlert(id) != nil {
				return fmt.Errorf("alert %s still exists", id)
			}
			return nil
		},
		Steps: []resource.TestStep{{
			Config: cfg(globalAgentID, 300, `"nc-1", "nc-2"`),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttrSet(name, "id"),
				resource.TestCheckResourceAttr(name, "agent_name", "probe-4-ams-1"),
				resource.TestCheckResourceAttr(name, "notification_channel_ids.#", "2"),
				stored(func(a *syntheticspb.AgentAlert) error {
					return all(eq("agent", a.GetAgentId(), globalAgentID), eq("threshold", a.GetThresholdSeconds(), uint32(300)))
				}),
			),
		}, {
			// In-place update.
			Config: cfg(globalAgentID, 600, `"nc-2"`),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr(name, "threshold_seconds", "600"),
				expectCalls(fake, "UpdateAgentAlert", 1),
				stored(func(a *syntheticspb.AgentAlert) error {
					return all(eq("threshold", a.GetThresholdSeconds(), uint32(600)),
						eq("channels", slices.Equal(a.GetNotificationChannelIds(), []string{"nc-2"}), true))
				}),
			),
		}, {
			ResourceName: name, ImportState: true, ImportStateVerify: true,
		}, {
			// Changing the agent replaces the alert.
			Config: cfg(privateAgentID, 600, `"nc-2"`),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr(name, "agent_id", privateAgentID),
				expectCalls(fake, "CreateAgentAlert", 2),
				expectCalls(fake, "DeleteAgentAlert", 1),
			),
		}},
	})
}

func TestDataSourceAgentAlerts(t *testing.T) {
	t.Parallel()
	fake := startFake(t)
	a1 := fake.AddAgentAlert(&syntheticspb.AgentAlert{AgentId: globalAgentID, ThresholdSeconds: 60, NotificationChannelIds: []string{"nc"}})
	fake.AddAgentAlert(&syntheticspb.AgentAlert{AgentId: privateAgentID, ThresholdSeconds: 120})
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: protoV5Factories(fake),
		Steps: []resource.TestStep{{
			Config: providerConfig + `
data "kentik-synthetics_agent_alerts" "all" {}
data "kentik-synthetics_agent_alerts" "global" { agent_ids = ["` + globalAgentID + `"] }
data "kentik-synthetics_agent_alert" "one" { id = "` + a1 + `" }`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("data.kentik-synthetics_agent_alerts.all", "items.#", "2"),
				resource.TestCheckResourceAttr("data.kentik-synthetics_agent_alerts.global", "items.#", "1"),
				resource.TestCheckResourceAttr("data.kentik-synthetics_agent_alerts.global", "items.0.threshold_seconds", "60"),
				resource.TestCheckResourceAttr("data.kentik-synthetics_agent_alert.one", "agent_id", globalAgentID),
				resource.TestCheckResourceAttr("data.kentik-synthetics_agent_alert.one", "notification_channel_ids.#", "1"),
			),
		}},
	})
}
