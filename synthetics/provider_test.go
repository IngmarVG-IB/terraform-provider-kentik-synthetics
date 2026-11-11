package synthetics_test

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/kentik/terraform-provider-kentik-synthetics/synthetics"
)

func TestProvider_InternalValidate(t *testing.T) {
	t.Parallel()
	require.NoError(t, synthetics.NewProvider().InternalValidate())
}

func TestProvider_SendsCredentials(t *testing.T) {
	t.Parallel()
	fake := startFake(t)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: protoV5Factories(fake),
		Steps: []resource.TestStep{{
			Config: providerConfig + `data "kentik-synthetics_agent" "a" { id = "968" }`,
		}},
	})
	calls := fake.Calls()
	require.NotEmpty(t, calls)
	for _, c := range calls {
		assert.Equal(t, testEmail, c.Email)
		assert.Equal(t, testToken, c.Token)
	}
}

func TestProvider_CustomRetryAndLegacyAPIURL(t *testing.T) {
	t.Parallel()
	fake := startFake(t)
	fake.FailNext("GetAgent", status.Error(codes.Unavailable, "busy"), status.Error(codes.ResourceExhausted, "slow down"))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: protoV5Factories(fake),
		Steps: []resource.TestStep{{
			Config: `
provider "kentik-synthetics" {
  email        = "` + testEmail + `"
  token        = "` + testToken + `"
  api_url      = "https://synthetics.api.kentik.com"
  timeout      = "30s"
  log_payloads = true
  retry {
    max_attempts = 3
    min_delay    = "1ms"
    max_delay    = "2ms"
  }
}
data "kentik-synthetics_agent" "a" { id = "968" }
`,
			Check: resource.TestCheckResourceAttr("data.kentik-synthetics_agent.a", "alias", "probe-4-ams-1"),
		}},
	})
}

func TestProvider_InvalidRetryConfig(t *testing.T) {
	t.Parallel()
	fake := startFake(t)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: protoV5Factories(fake),
		Steps: []resource.TestStep{{
			Config: `
provider "kentik-synthetics" {
  email = "a@example.com"
  token = "t"
  retry { max_delay = "invalid-delay" }
}
data "kentik-synthetics_agent" "a" { id = "968" }
`,
			ExpectError: regexp.MustCompile(`invalid duration "invalid-delay"`),
		}},
	})
}

func TestProvider_APIErrorsAreReported(t *testing.T) {
	t.Parallel()
	fake := startFake(t)
	fake.FailNext("GetAgent", status.Error(codes.PermissionDenied, "nope"))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: protoV5Factories(fake),
		Steps: []resource.TestStep{{
			Config:      providerConfig + `data "kentik-synthetics_agent" "a" { id = "968" }`,
			ExpectError: regexp.MustCompile(`(?s)PermissionDenied.*lacks the permission`),
		}},
	})
}
