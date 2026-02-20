package main

import (
	"os"

	"github.com/jakeschepis/zeus-cli/internal/cli"
)

// version is set at build time via -ldflags "-X main.version=x.y.z"
var version = "dev"

func main() {
	if err := cli.Execute(version); err != nil {
		os.Exit(1)
	}
}
