// Command terraform-provider-tplink is a Terraform/OpenTofu provider for
// TP-Link home routers running the new-generation (SG-hardened) web UI.
package main

import (
	"context"
	"flag"
	"log"

	frameworkprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"

	"github.com/repparw/terraform-provider-tplink/internal/provider"
)

// version is set at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	var debug bool
	flag.BoolVar(&debug, "debug", false, "set to true to run the provider with support for debuggers like delve")
	flag.Parse()

	opts := providerserver.ServeOpts{
		Address:         "registry.opentofu.org/repparw/tplink",
		Debug:           debug,
		ProtocolVersion: 6,
	}
	if err := providerserver.Serve(context.Background(), func() frameworkprovider.Provider { return provider.New(version) }, opts); err != nil {
		log.Fatal(err.Error())
	}
}
