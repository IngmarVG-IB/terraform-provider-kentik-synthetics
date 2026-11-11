package synthetics_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	syntheticspb "github.com/kentik/api-schema-public/gen/go/kentik/synthetics/v202309"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/kentik/terraform-provider-kentik-synthetics/internal/fakesynthetics"
	"github.com/kentik/terraform-provider-kentik-synthetics/internal/kentikapi"
	"github.com/kentik/terraform-provider-kentik-synthetics/synthetics"
)

const (
	testEmail = "terraform-test@example.com"
	testToken = "dummy-token"

	// providerConfig is prepended to every unit test configuration.
	providerConfig = `
provider "kentik-synthetics" {
  email = "` + testEmail + `"
  token = "` + testToken + `"
}
`
	globalAgentID  = "968"
	privateAgentID = "1717"
)

// startFake starts a fake API seeded with two agents.
func startFake(t *testing.T) *fakesynthetics.Server {
	t.Helper()
	fake := fakesynthetics.Start(t)
	fake.AddAgent(&syntheticspb.Agent{
		Id: globalAgentID, SiteName: "global-agent", Status: syntheticspb.AgentStatus_AGENT_STATUS_OK,
		Alias: "probe-4-ams-1", Type: "global", Os: "Linux", Lat: 52.374031, Long: 4.88969,
		LastAuthed: timestamppb.New(time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)),
		Family:     syntheticspb.IPFamily_IP_FAMILY_DUAL, Asn: 20473, SiteId: "0", Version: "1.2.3",
		City: "Amsterdam", Region: "Noord-Holland", Country: "Netherlands", TestIds: []string{"13", "1337"},
		CloudRegion: "", CloudProvider: "", AgentImpl: syntheticspb.ImplementType_IMPLEMENT_TYPE_RUST,
		Labels: []string{"eu", "global"},
		Metadata: &syntheticspb.AgentMetadata{
			PublicIpv4Addresses: []*syntheticspb.AgentMetadata_IpValue{{Value: "192.0.2.10"}},
			StaticIpv4:          true,
		},
	})
	fake.AddAgent(&syntheticspb.Agent{
		Id: privateAgentID, SiteName: "office", Status: syntheticspb.AgentStatus_AGENT_STATUS_WAIT,
		Alias: "office-agent", Type: "private", Lat: 40.7128, Long: -74.006,
		Family: syntheticspb.IPFamily_IP_FAMILY_V4, Asn: 64500, SiteId: "42", Version: "1.2.3",
		City: "New York", Region: "New York", Country: "United States",
		AgentImpl: syntheticspb.ImplementType_IMPLEMENT_TYPE_NODE,
	})
	return fake
}

// protoV5Factories returns provider factories that connect to fake.
func protoV5Factories(fake *fakesynthetics.Server) map[string]func() (tfprotov5.ProviderServer, error) {
	return map[string]func() (tfprotov5.ProviderServer, error){
		"kentik-synthetics": func() (tfprotov5.ProviderServer, error) {
			p := synthetics.New("test", func(cfg *kentikapi.Config) {
				cfg.Endpoint = fake.Endpoint()
				cfg.Insecure = true
				cfg.DialOptions = fake.DialOptions()
				cfg.Retry.MinDelay = time.Millisecond
				cfg.Retry.MaxDelay = 5 * time.Millisecond
			})()
			return p.GRPCProvider(), nil
		},
	}
}

// checkTestStored asserts something about the test stored in the fake for resource name.
func checkTestStored(
	fake *fakesynthetics.Server, name string, check func(*syntheticspb.Test) error,
) func(*terraform.State) error {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf("resource %s not in state", name)
		}
		t := fake.Test(rs.Primary.ID)
		if t == nil {
			return fmt.Errorf("test %s not found in fake API", rs.Primary.ID)
		}
		return check(t)
	}
}
