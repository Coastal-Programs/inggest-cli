package main

import (
	"os"

	"github.com/jakeschepis/zeus-cli/internal/auth"
	"github.com/jakeschepis/zeus-cli/internal/cli"
)

// Injected at build time via -ldflags:
//
//	-X main.version=v0.1.0
//	-X main.defaultClientID=<your-xero-client-id>
//	-X main.proxyURL=https://zeus-auth-proxy.<subdomain>.workers.dev
var (
	version         = "dev"
	defaultClientID = ""
	proxyURL        = ""
)

func main() {
	auth.DefaultClientID = defaultClientID
	auth.ProxyURL = proxyURL

	if err := cli.Execute(version); err != nil {
		os.Exit(1)
	}
}
