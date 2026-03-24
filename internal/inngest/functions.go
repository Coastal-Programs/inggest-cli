package inngest

import (
	"context"
	"fmt"
)

const listFunctionsQuery = `query {
  functions {
    id
    name
    slug
    config
    url
    appID
    concurrency
    triggers {
      type
      value
      condition
    }
    configuration {
      retries { value isDefault }
      concurrency { scope limit { value } key }
      rateLimit { limit period key }
      debounce { period key }
      throttle { burst limit period key }
      eventsBatch { maxSize timeout key }
      priority
    }
    app {
      id
      name
      sdkLanguage
      sdkVersion
      framework
      url
      connected
    }
  }
}`

const getFunctionQuery = `query GetFunction($slug: String!) {
  functionBySlug(query: {functionSlug: $slug}) {
    id
    name
    slug
    config
    url
    appID
    concurrency
    triggers {
      type
      value
      condition
    }
    configuration {
      retries { value isDefault }
      concurrency { scope limit { value } key }
      rateLimit { limit period key }
      debounce { period key }
      throttle { burst limit period key }
      eventsBatch { maxSize timeout key }
      priority
    }
    app {
      id
      name
      sdkLanguage
      sdkVersion
      framework
      url
      connected
    }
  }
}`

// ListFunctions queries all functions via GraphQL.
func (c *Client) ListFunctions(ctx context.Context) ([]Function, error) {
	var result struct {
		Functions []Function `json:"functions"`
	}
	if err := c.ExecuteGraphQL(ctx, "ListFunctions", listFunctionsQuery, nil, &result); err != nil {
		return nil, err
	}
	return result.Functions, nil
}

// GetFunction gets a function by slug via GraphQL.
func (c *Client) GetFunction(ctx context.Context, slug string) (*Function, error) {
	vars := map[string]any{
		"slug": slug,
	}
	var result struct {
		FunctionBySlug *Function `json:"functionBySlug"`
	}
	if err := c.ExecuteGraphQL(ctx, "GetFunction", getFunctionQuery, vars, &result); err != nil {
		return nil, err
	}
	if result.FunctionBySlug == nil {
		return nil, fmt.Errorf("inngest: function %q not found", slug)
	}
	return result.FunctionBySlug, nil
}
