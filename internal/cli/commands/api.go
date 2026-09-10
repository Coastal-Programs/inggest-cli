package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Coastal-Programs/inggest-cli/internal/inngest"
)

// NewAPICmd returns the "api" passthrough command.
func NewAPICmd() *cobra.Command {
	var (
		method   string
		body     string
		bodyFile string
		raw      bool
	)

	cmd := &cobra.Command{
		Use:   "api <path>",
		Short: "Call any Inngest REST API endpoint with your credentials",
		Long: `Make an authenticated request to the Inngest REST API and print the response.

The path is relative to the API base URL and may include a query string, e.g.
  inngest api /v2/runs?limit=5
  inngest api /v2/runs/<run-id>/cancel -X POST
  inngest api /v2/events -X POST --body '{"name":"user.signup","data":{}}'

Reference: https://api-docs.inngest.com. Exits non-zero on 4xx/5xx responses.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := newCloudClient()

			payload, err := readBodyInput(body, bodyFile)
			if err != nil {
				return err
			}
			if len(payload) > 0 && method == http.MethodGet {
				method = http.MethodPost
			}

			resp, err := client.RawRequest(cmd.Context(), method, args[0], payload)
			if err != nil {
				return err
			}

			out := resp.Body
			if !raw && json.Valid(out) {
				var buf bytes.Buffer
				if json.Indent(&buf, out, "", "  ") == nil {
					out = buf.Bytes()
				}
			}
			if len(out) > 0 {
				fmt.Fprintln(cmd.OutOrStdout(), strings.TrimRight(string(out), "\n"))
			}

			if resp.StatusCode >= http.StatusBadRequest {
				return apiStatusError(resp)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&method, "method", "X", http.MethodGet, "HTTP method (GET, POST, PUT, PATCH, DELETE)")
	cmd.Flags().StringVar(&body, "body", "", "Request body as a JSON string")
	cmd.Flags().StringVar(&bodyFile, "body-file", "", `Read the request body from a file ("-" for stdin)`)
	cmd.Flags().BoolVar(&raw, "raw", false, "Print the response body exactly as received")

	return cmd
}

// readBodyInput returns the request body from --body or --body-file ("-" = stdin).
func readBodyInput(flagBody, flagFile string) ([]byte, error) {
	switch {
	case flagBody != "":
		return []byte(flagBody), nil
	case flagFile == "-":
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("reading stdin: %w", err)
		}
		return b, nil
	case flagFile != "":
		b, err := os.ReadFile(flagFile)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", flagFile, err)
		}
		return b, nil
	}
	return nil, nil
}

// apiStatusError turns a non-2xx passthrough response into an APIError so the
// exit code reflects auth failures.
func apiStatusError(resp *inngest.RawResponse) error {
	apiErr := &inngest.APIError{StatusCode: resp.StatusCode, Message: http.StatusText(resp.StatusCode)}
	var envelope struct {
		Errors []struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"errors"`
		Error string `json:"error"`
	}
	if json.Unmarshal(resp.Body, &envelope) == nil {
		switch {
		case len(envelope.Errors) > 0:
			apiErr.Code = envelope.Errors[0].Code
			apiErr.Message = envelope.Errors[0].Message
		case envelope.Error != "":
			apiErr.Message = envelope.Error
		}
	}
	return apiErr
}
