package commands

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/Coastal-Programs/inggest-cli/internal/cli/state"
	"github.com/Coastal-Programs/inggest-cli/internal/common/config"
	"github.com/Coastal-Programs/inggest-cli/internal/inngest"
	"github.com/Coastal-Programs/inggest-cli/pkg/output"
)

const sourceConfigEnvAlsoSet = "config (env var also set)"

// NewAuthCmd returns the "auth" command group.
func NewAuthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage authentication",
		Long:  "Log in, log out, and check authentication status for Inngest.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newAuthLoginCmd())
	cmd.AddCommand(newAuthLogoutCmd())
	cmd.AddCommand(newAuthStatusCmd())
	return cmd
}

// resolveSigningKey resolves the signing key from flag, env var, or interactive prompt.
func resolveSigningKey(flagValue string) (string, error) {
	key := flagValue
	if key == "" {
		key = os.Getenv("INNGEST_SIGNING_KEY")
	}
	if key == "" {
		if !isInteractive() {
			return "", fmt.Errorf("signing key required: use --signing-key flag or INNGEST_SIGNING_KEY env var")
		}
		var err error
		key, err = readSecret("Enter signing key: ")
		if err != nil {
			return "", err
		}
	}
	key = strings.TrimSpace(key)
	if err := validateSigningKey(key); err != nil {
		return "", fmt.Errorf("invalid signing key: %w", err)
	}
	return key, nil
}

func newAuthLoginCmd() *cobra.Command {
	var signingKey string
	var signingKeyFallback string
	var eventKey string

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate with Inngest",
		Long:  "Save your Inngest signing key (and optionally event key) to the CLI config.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := state.Config
			format := output.Format(state.Output)

			var err error
			signingKey, err = resolveSigningKey(signingKey)
			if err != nil {
				return err
			}

			// Resolve event key: flag > env > prompt
			if eventKey == "" {
				eventKey = os.Getenv("INNGEST_EVENT_KEY")
			}
			if eventKey == "" && isInteractive() {
				eventKey, _ = readSecret("Enter event key (optional, press Enter to skip): ")
			}

			cfg.SigningKey = signingKey
			if eventKey != "" {
				cfg.EventKey = strings.TrimSpace(eventKey)
			}

			// Resolve signing key fallback: flag > env
			if signingKeyFallback == "" {
				signingKeyFallback = os.Getenv("INNGEST_SIGNING_KEY_FALLBACK")
			}
			if signingKeyFallback != "" {
				signingKeyFallback = strings.TrimSpace(signingKeyFallback)
				if err := validateSigningKey(signingKeyFallback); err != nil {
					return fmt.Errorf("invalid signing key fallback: %w", err)
				}
				cfg.SigningKeyFallback = signingKeyFallback
			}

			if err := cfg.Save(); err != nil {
				return fmt.Errorf("saving config: %w", err)
			}

			result := map[string]string{
				"status":      "authenticated",
				"signing_key": config.Redact(signingKey),
			}
			if cfg.SigningKeyFallback != "" {
				result["signing_key_fallback"] = config.Redact(cfg.SigningKeyFallback)
			}
			if cfg.EventKey != "" {
				result["event_key"] = config.Redact(cfg.EventKey)
			}

			return output.Print(result, format)
		},
	}

	cmd.Flags().StringVar(&signingKey, "signing-key", "", "Inngest signing key")
	cmd.Flags().StringVar(&signingKeyFallback, "signing-key-fallback", "", "Inngest signing key fallback (for key rotation)")
	cmd.Flags().StringVar(&eventKey, "event-key", "", "Inngest event key (for sending events)")

	return cmd
}

func newAuthLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Clear stored credentials",
		Long:  "Remove signing key and event key from the CLI config.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := state.Config
			format := output.Format(state.Output)

			cfg.SigningKey = ""
			cfg.SigningKeyFallback = ""
			cfg.EventKey = ""

			if err := cfg.Save(); err != nil {
				return fmt.Errorf("saving config: %w", err)
			}

			return output.Print(map[string]string{
				"status":  "logged_out",
				"message": "Credentials cleared",
			}, format)
		},
	}
}

// populateKeyStatus adds key info and source to the result map.
func populateKeyStatus(result map[string]any, keyName, keyValue, envVarValue, configValue, envVarLabel string) {
	if keyValue != "" {
		result[keyName] = config.Redact(keyValue)
		switch {
		case envVarValue != "" && configValue != "":
			result[keyName+"_source"] = sourceConfigEnvAlsoSet
		case envVarValue != "":
			result[keyName+"_source"] = "env (" + envVarLabel + ")"
		default:
			result[keyName+"_source"] = sourceConfig
		}
	} else {
		result[keyName] = "not configured"
	}
}

// validateAPIConnection checks if the signing key is valid by querying the API.
func validateAPIConnection(result map[string]any, signingKey, signingKeyFallback string) {
	client := inngest.NewClient(inngest.ClientOptions{
		SigningKey:         signingKey,
		SigningKeyFallback: signingKeyFallback,
		Env:                state.Env,
		APIBaseURL:         state.APIBaseURL,
		DevServerURL:       state.DevServer,
		DevMode:            state.DevMode,
		UserAgent:          "inngest-cli/" + state.AppVersion,
	})

	var data any
	err := client.ExecuteGraphQL(
		context.Background(),
		"AuthCheck",
		`query AuthCheck { __typename }`,
		nil,
		&data,
	)
	if err != nil {
		result["api_validation"] = "failed"
		result["api_validation_error"] = err.Error()
	} else {
		result["api_validation"] = "ok"
	}
}

func newAuthStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show authentication status",
		Long:  "Display current authentication state, environment, and API configuration.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := state.Config
			format := output.Format(state.Output)

			signingKey := cfg.GetSigningKey()
			signingKeyFallback := cfg.GetSigningKeyFallback()
			eventKey := cfg.GetEventKey()

			result := map[string]any{
				"authenticated":  signingKey != "",
				"environment":    state.Env,
				"api_base_url":   state.APIBaseURL,
				"dev_server_url": state.DevServer,
			}

			populateKeyStatus(result, "signing_key", signingKey, os.Getenv("INNGEST_SIGNING_KEY"), cfg.SigningKey, "INNGEST_SIGNING_KEY")
			populateKeyStatus(result, "signing_key_fallback", signingKeyFallback, os.Getenv("INNGEST_SIGNING_KEY_FALLBACK"), cfg.SigningKeyFallback, "INNGEST_SIGNING_KEY_FALLBACK")
			populateKeyStatus(result, "event_key", eventKey, os.Getenv("INNGEST_EVENT_KEY"), cfg.EventKey, "INNGEST_EVENT_KEY")

			// Custom API URL indicator
			if cfg.APIBaseURL != "" {
				result["custom_api_url"] = true
			}

			if signingKey != "" {
				validateAPIConnection(result, signingKey, signingKeyFallback)
			}

			return output.Print(result, format)
		},
	}
}

// isInteractiveFn is the function used to check if stdin is a terminal.
// Overridden in tests to simulate interactive/non-interactive modes.
var isInteractiveFn = func() bool {
	return term.IsTerminal(syscall.Stdin)
}

// isInteractive returns true if stdin is a terminal.
func isInteractive() bool {
	return isInteractiveFn()
}

// termReadPasswordFn wraps term.ReadPassword for test injection.
var termReadPasswordFn = term.ReadPassword

// readSecretFn is the function used to read a secret from the terminal.
// Overridden in tests to avoid requiring a real terminal.
var readSecretFn = defaultReadSecret

func defaultReadSecret(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)
	bytes, err := termReadPasswordFn(syscall.Stdin)
	fmt.Fprintln(os.Stderr) // newline after hidden input
	if err != nil {
		return "", fmt.Errorf("failed to read input: %w", err)
	}
	return strings.TrimSpace(string(bytes)), nil
}

// readSecret prompts for a secret without echoing to terminal.
func readSecret(prompt string) (string, error) {
	return readSecretFn(prompt)
}

// validateSigningKey checks if the key is a valid Inngest signing key.
// Accepts two formats:
// 1. Inngest Cloud: starts with "signkey-" prefix (e.g. signkey-prod-xxx, signkey-test-xxx)
// 2. Self-hosted: valid hex string with even number of characters
func validateSigningKey(key string) error {
	if key == "" {
		return fmt.Errorf("signing key cannot be empty")
	}

	// Cloud format: signkey-{env}-{hex}
	if strings.HasPrefix(key, "signkey-") {
		// Strip the signkey-{env}- prefix and validate the hex portion
		parts := strings.SplitN(key, "-", 3)
		if len(parts) < 3 || parts[2] == "" {
			return fmt.Errorf("signing key must have format signkey-{env}-{hex}")
		}
		if len(parts[2])%2 != 0 {
			return fmt.Errorf("signing key hex portion must have even length")
		}
		if _, err := hex.DecodeString(parts[2]); err != nil {
			return fmt.Errorf("signing key hex portion is not valid hex: %w", err)
		}
		return nil
	}

	// Self-hosted format: raw hex string
	if len(key)%2 != 0 {
		return fmt.Errorf("signing key must be a signkey-* prefixed key (Inngest Cloud) or a hex string with even length (self-hosted)")
	}
	if _, err := hex.DecodeString(key); err != nil {
		return fmt.Errorf("signing key must be a signkey-* prefixed key (Inngest Cloud) or a valid hex string (self-hosted): %w", err)
	}

	return nil
}
