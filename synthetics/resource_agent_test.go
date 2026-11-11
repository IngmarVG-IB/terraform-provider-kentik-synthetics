package synthetics_test

import (
	"fmt"
	"regexp"
	"slices"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	syntheticspb "github.com/kentik/api-schema-public/gen/go/kentik/synthetics/v202309"

	"github.com/kentik/terraform-provider-kentik-synthetics/internal/fakesynthetics"
)

func agentStored(fake *fakesynthetics.Server, id string, check func(*syntheticspb.Agent) error) resource.TestCheckFunc {
	return func(*terraform.State) error {
		a := fake.Agent(id)
		if a == nil {
			return fmt.Errorf("agent %s not found in fake API", id)
		}
		return check(a)
	}
}

func TestResourceAgent_AdoptUpdateImportForget(t *testing.T) {
	t.Parallel()
	fake := startFake(t)
	const name = "kentik-synthetics_agent.office"
	cfg := func(alias, labels string) string {
		return providerConfig + fmt.Sprintf(`
resource "kentik-synthetics_agent" "office" {
  agent_id = "%s"
  alias    = %q
  status   = "AGENT_STATUS_OK"
  labels   = [%s]
}`, privateAgentID, alias, labels)
	}
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: protoV5Factories(fake),
		// delete_on_destroy defaults to false: the agent must survive terraform destroy.
		CheckDestroy: agentStored(fake, privateAgentID, func(a *syntheticspb.Agent) error {
			return eq("alias after destroy", a.GetAlias(), "office-ny")
		}),
		Steps: []resource.TestStep{{
			Config: cfg("office-agent-2", `"office", "ny"`),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr(name, "id", privateAgentID),
				resource.TestCheckResourceAttr(name, "alias", "office-agent-2"),
				resource.TestCheckResourceAttr(name, "status", "AGENT_STATUS_OK"),
				resource.TestCheckResourceAttr(name, "labels.#", "2"),
				// Unconfigured attributes are read from the API and left unchanged.
				resource.TestCheckResourceAttr(name, "site_name", "office"),
				resource.TestCheckResourceAttr(name, "city", "New York"),
				resource.TestCheckResourceAttr(name, "type", "private"),
				agentStored(fake, privateAgentID, func(a *syntheticspb.Agent) error {
					return all(
						eq("status", a.GetStatus(), syntheticspb.AgentStatus_AGENT_STATUS_OK),
						eq("alias", a.GetAlias(), "office-agent-2"),
						eq("labels", slices.Equal(a.GetLabels(), []string{"office", "ny"}) ||
							slices.Equal(a.GetLabels(), []string{"ny", "office"}), true),
						eq("site_name unchanged", a.GetSiteName(), "office"),
						eq("lat unchanged", a.GetLat(), 40.7128),
					)
				}),
			),
		}, {
			Config: cfg("office-ny", `"office"`),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr(name, "alias", "office-ny"),
				resource.TestCheckResourceAttr(name, "labels.#", "1"),
				expectCalls(fake, "UpdateAgent", 2),
			),
		}, {
			ResourceName:            name,
			ImportState:             true,
			ImportStateId:           privateAgentID,
			ImportStateVerify:       true,
			ImportStateVerifyIgnore: []string{"delete_on_destroy"},
		}},
	})
	if fake.CallCount("DeleteAgent") != 0 {
		t.Fatalf("DeleteAgent called without delete_on_destroy")
	}
}

func TestResourceAgent_DeleteOnDestroy(t *testing.T) {
	t.Parallel()
	fake := startFake(t)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: protoV5Factories(fake),
		CheckDestroy: func(*terraform.State) error {
			if fake.Agent(privateAgentID) != nil {
				return fmt.Errorf("agent still exists")
			}
			return nil
		},
		Steps: []resource.TestStep{{
			Config: providerConfig + `
resource "kentik-synthetics_agent" "a" {
  agent_id          = "` + privateAgentID + `"
  delete_on_destroy = true
}`,
			// Nothing configured to change: adopting does not call UpdateAgent.
			Check: expectCalls(fake, "UpdateAgent", 0),
		}},
	})
}

func TestResourceAgent_UnknownAgent(t *testing.T) {
	t.Parallel()
	fake := startFake(t)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: protoV5Factories(fake),
		Steps: []resource.TestStep{{
			Config:      providerConfig + `resource "kentik-synthetics_agent" "a" { agent_id = "404" }`,
			ExpectError: regexp.MustCompile(`agents cannot be created through the Kentik API`),
		}, {
			Config: providerConfig + `
resource "kentik-synthetics_agent" "a" {
  agent_id = "968"
  status   = "AGENT_STATUS_DELETED"
}`,
			PlanOnly:    true,
			ExpectError: regexp.MustCompile(`expected status to be one of`),
		}},
	})
}
