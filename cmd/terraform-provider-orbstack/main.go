package main

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/machugram/terraform-provider-orbstack/internal/provider"
)

var (
	version = "dev"
	// commit is set by goreleaser at release time.
	commit = "none"
)

func main() {
	var debug bool
	flag.BoolVar(&debug, "debug", false, "run the provider under a debugger")
	flag.Parse()

	err := providerserver.Serve(context.Background(), provider.New(version), providerserver.ServeOpts{
		Address: "registry.terraform.io/machugram/orbstack",
		Debug:   debug,
	})
	if err != nil {
		log.Fatal(err)
	}
}
