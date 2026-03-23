package inngest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// graphqlRequest is the JSON body sent to the GraphQL endpoint.
type graphqlRequest struct {
	OperationName string                 `json:"operationName,omitempty"`
	Query         string                 `json:"query"`
	Variables     map[string]interface{} `json:"variables,omitempty"`
}

// graphqlResponse is the JSON body returned from the GraphQL endpoint.
type graphqlResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []graphqlError  `json:"errors,omitempty"`
}

// graphqlError represents a single GraphQL error.
type graphqlError struct {
	Message string `json:"message"`
}

// ExecuteGraphQL sends a GraphQL query and unmarshals the response data into result.
// operationName identifies the operation for server-side logging and debugging.
func (c *Client) ExecuteGraphQL(ctx context.Context, operationName string, query string, variables map[string]interface{}, result interface{}) error {
	body, err := json.Marshal(graphqlRequest{
		OperationName: operationName,
		Query:         query,
		Variables:     variables,
	})
	if err != nil {
		return fmt.Errorf("inngest: marshal graphql request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.graphqlURL(), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("inngest: create graphql request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.do(req)
	if err != nil {
		return fmt.Errorf("inngest: graphql request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("inngest: read graphql response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("inngest: graphql returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var gqlResp graphqlResponse
	if err := json.Unmarshal(respBody, &gqlResp); err != nil {
		return fmt.Errorf("inngest: unmarshal graphql response: %w", err)
	}

	if len(gqlResp.Errors) > 0 {
		msgs := make([]string, len(gqlResp.Errors))
		for i, e := range gqlResp.Errors {
			msgs[i] = e.Message
		}
		return fmt.Errorf("inngest: graphql errors: %s", strings.Join(msgs, "; "))
	}

	if result != nil && gqlResp.Data != nil {
		if err := json.Unmarshal(gqlResp.Data, result); err != nil {
			return fmt.Errorf("inngest: unmarshal graphql data: %w", err)
		}
	}

	return nil
}
