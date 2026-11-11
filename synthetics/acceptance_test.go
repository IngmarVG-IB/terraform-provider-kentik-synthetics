package synthetics_test

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/kentik/terraform-provider-kentik-synthetics/synthetics"
)

// Acceptance tests run against a live Kentik tenant. They are skipped unless TF_ACC=1 and need
// KTAPI_AUTH_EMAIL, KTAPI_AUTH_TOKEN and KTAPI_ACC_AGENT_ID (an agent the test may use). Run them with
// `make testacc`. They create and destroy real synthetic tests.

func accPreCheck(t *testing.T) {
	t.Helper()
	for _, v := range []string{"KTAPI_AUTH_EMAIL", "KTAPI_AUTH_TOKEN", "KTAPI_ACC_AGENT_ID"} {
		if os.Getenv(v) == "" {
			t.Fatalf("%s must be set for acceptance tests", v)
		}
	}
}

func accFactories() map[string]func() (tfprotov5.ProviderServer, error) {
	return map[string]func() (tfprotov5.ProviderServer, error){
		"kentik-synthetics": func() (tfprotov5.ProviderServer, error) {
			return synthetics.New("acceptance")().GRPCProvider(), nil
		},
	}
}

func TestAccResourceTest_Hostname(t *testing.T) {
	agentID := os.Getenv("KTAPI_ACC_AGENT_ID")
	cfg := func(status string) string {
		return `
resource "kentik-synthetics_test" "acc" {
  name   = "tf-acc-hostname"
  type   = "hostname"
  status = "` + status + `"
  settings {
    hostname { target = "www.example.com" }
    agent_ids = ["` + agentID + `"]
    tasks     = ["ping", "traceroute"]
  }
}`
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV5ProviderFactories: accFactories(),
		Steps: []resource.TestStep{
			{Config: cfg("TEST_STATUS_ACTIVE"), Check: resource.TestCheckResourceAttrSet("kentik-synthetics_test.acc", "id")},
			{Config: cfg("TEST_STATUS_PAUSED"), Check: resource.TestCheckResourceAttr("kentik-synthetics_test.acc", "status", "TEST_STATUS_PAUSED")},
			{ResourceName: "kentik-synthetics_test.acc", ImportState: true, ImportStateVerify: true},
		},
	})
}

func TestAccDataSourceAgents(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV5ProviderFactories: accFactories(),
		Steps: []resource.TestStep{{
			Config: `data "kentik-synthetics_agents" "all" {}`,
			Check:  resource.TestCheckResourceAttrSet("data.kentik-synthetics_agents.all", "items.#"),
		}},
	})
}
