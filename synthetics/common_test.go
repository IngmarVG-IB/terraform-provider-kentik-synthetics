package synthetics_test

import (
	"net/http"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/stretchr/testify/require"

	"github.com/kentik/terraform-provider-kentik-synthetics/synthetics"
)

func checkAPIServerConnection(t *testing.T) {
	apiURL, ok := os.LookupEnv("KTAPI_URL")
	require.True(t, ok, "KTAPI_URL env variable not set")

	_, err := http.Get(apiURL) //nolint:bodyclose,noctx // test-only connectivity probe
	require.NoErrorf(t, err, "failed to connect to the API Server on URL %q", apiURL)
}

func providerFactories() map[string]func() (*schema.Provider, error) {
	return map[string]func() (*schema.Provider, error){
		"kentik-synthetics": func() (*schema.Provider, error) {
			return synthetics.NewProvider(), nil
		},
	}
}
