//go:generate go tool tfplugindocs generate --provider-name kentik-synthetics

package main

import (
	"flag"

	"github.com/hashicorp/terraform-plugin-sdk/v2/plugin"

	"github.com/kentik/terraform-provider-kentik-synthetics/synthetics"
)

// version is set by goreleaser via -ldflags and reported in the API user agent.
var version = "dev"

func main() {
	var debugMode bool
	flag.BoolVar(&debugMode, "debug", false, "set to true to run the provider with support for debuggers like Delve")
	flag.Parse()

	opts := &plugin.ServeOpts{
		ProviderFunc: synthetics.New(version),
		ProviderAddr: "registry.terraform.io/kentik/kentik-synthetics",
		Debug:        debugMode,
	}

	plugin.Serve(opts)
}
