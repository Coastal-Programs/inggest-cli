package inngest

import (
	"context"
	"fmt"
)

const listAppsQuery = `query {
  apps {
    id
    externalID
    name
    sdkLanguage
    sdkVersion
    framework
    url
    checksum
    error
    connected
    functionCount
    autodiscovered
    method
    functions {
      id
      name
      slug
    }
  }
}`

const getAppQuery = `query GetApp($id: UUID!) {
  app(id: $id) {
    id
    externalID
    name
    sdkLanguage
    sdkVersion
    framework
    url
    checksum
    error
    connected
    functionCount
    method
    functions {
      id
      name
      slug
      triggers {
        type
        value
      }
    }
  }
}`

// ListApps queries all apps (which represent environments/deployments) via GraphQL.
func (c *Client) ListApps(ctx context.Context) ([]App, error) {
	var result struct {
		Apps []App `json:"apps"`
	}
	if err := c.ExecuteGraphQL(ctx, "ListApps", listAppsQuery, nil, &result); err != nil {
		return nil, err
	}
	return result.Apps, nil
}

// GetApp gets a single app by ID.
func (c *Client) GetApp(ctx context.Context, appID string) (*App, error) {
	vars := map[string]interface{}{
		"id": appID,
	}
	var result struct {
		App *App `json:"app"`
	}
	if err := c.ExecuteGraphQL(ctx, "GetApp", getAppQuery, vars, &result); err != nil {
		return nil, err
	}
	if result.App == nil {
		return nil, fmt.Errorf("inngest: app %q not found", appID)
	}
	return result.App, nil
}
