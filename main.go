//go:generate go tool tfplugindocs generate --provider-name kentik-synthetics

package main

import (
	"flag"

	"github.com/hashicorp/terraform-plugin-sdk/v2/plugin"

	"github.com/kentik/terraform-provider-kentik-synthetics/synthetics"
)

// Set by goreleaser via -ldflags.
var (
	version = "dev"
	commit  = "none"
)

func main() {
	var debugMode bool
	flag.BoolVar(&debugMode, "debug", false, "set to true to run the provider with support for debuggers like Delve")
	flag.Parse()

	opts := &plugin.ServeOpts{
		ProviderFunc: synthetics.NewProvider,
		ProviderAddr: "registry.terraform.io/kentik/kentik-synthetics",
		Debug:        debugMode,
	}
	_ = commit
	_ = version

	plugin.Serve(opts)
}
