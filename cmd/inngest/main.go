package main

import (
	"os"

	"github.com/Coastal-Programs/inggest-cli/internal/cli"
)

// Injected at build time via -ldflags:
//
//	-X main.version=v0.1.0
var version = "dev"

func main() {
	os.Exit(cli.Execute(version))
}
