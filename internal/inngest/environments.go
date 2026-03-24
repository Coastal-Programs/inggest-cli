package inngest

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// ErrAccountAuthRequired indicates that the query requires account-level
// authentication (e.g. an API key) rather than a signing key.
var ErrAccountAuthRequired = errors.New("account-level authentication required")

const listEnvsQuery = `query ListEnvs {
  envs {
    edges {
      node {
        id
        name
        slug
        type
        isAutoArchiveEnabled
        webhookSigningKey
        createdAt
      }
    }
    pageInfo {
      hasNextPage
      endCursor
    }
  }
}`

// ListEnvironments queries all environments via the envs GraphQL query.
// Note: this query requires account-level auth. Signing keys (which authenticate
// at the environment level) will receive an UNAUTHENTICATED error.
func (c *Client) ListEnvironments(ctx context.Context) ([]Environment, error) {
	var result struct {
		Envs EnvsConnection `json:"envs"`
	}
	if err := c.ExecuteGraphQL(ctx, "ListEnvs", listEnvsQuery, nil, &result); err != nil {
		if isUnauthenticatedError(err) {
			return nil, fmt.Errorf("%w: the envs query requires account-level authentication — "+
				"a signing key only grants access within its own environment. "+
				"View your environments at https://app.inngest.com/env", ErrAccountAuthRequired)
		}
		return nil, err
	}

	envs := make([]Environment, len(result.Envs.Edges))
	for i, edge := range result.Envs.Edges {
		envs[i] = edge.Node
	}
	return envs, nil
}

// GetEnvironment fetches a single environment by ID or name/slug.
// It lists all environments and filters client-side.
func (c *Client) GetEnvironment(ctx context.Context, nameOrID string) (*Environment, error) {
	envs, err := c.ListEnvironments(ctx)
	if err != nil {
		return nil, err
	}

	for i := range envs {
		if envs[i].ID == nameOrID ||
			strings.EqualFold(envs[i].Name, nameOrID) ||
			strings.EqualFold(envs[i].Slug, nameOrID) {
			return &envs[i], nil
		}
	}

	return nil, fmt.Errorf("inngest: environment %q not found", nameOrID)
}

// isUnauthenticatedError checks if a GraphQL error indicates an auth failure.
func isUnauthenticatedError(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unauthenticated") || strings.Contains(msg, "unauthorized")
}
