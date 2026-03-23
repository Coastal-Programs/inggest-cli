// Package state holds resolved CLI state shared between root and subcommands.
package state

import "github.com/Coastal-Programs/inggest-cli/internal/common/config"

var (
	AppVersion string
	Config     *config.Config
	Env        string
	APIBaseURL string
	DevServer  string
	DevMode    bool
	Output     string
)
