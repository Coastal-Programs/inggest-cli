package inngest

import (
	"context"
	"fmt"
)

// workflowFields is the shared set of fields queried for workflows (functions).
const workflowFields = `
        id name slug url isPaused isArchived
        triggers { type value condition }
        configuration {
          retries { value isDefault }
          concurrency { scope limit { value } key }
          rateLimit { limit period key }
          debounce { period key }
          throttle { burst limit period key }
          eventsBatch { maxSize timeout key }
          priority
        }
        app { id name externalID appVersion }
`

const listFunctionsQuery = `query ListFunctions {
  events(query: {}) {
    data {
      workflows {` + workflowFields + `
      }
    }
    page { page totalPages }
  }
}`

// ListFunctions queries all functions by collecting workflows from all event types
// and deduplicating by ID.
func (c *Client) ListFunctions(ctx context.Context) ([]Function, error) {
	var result struct {
		Events struct {
			Data []struct {
				Workflows []Function `json:"workflows"`
			} `json:"data"`
		} `json:"events"`
	}
	if err := c.ExecuteGraphQL(ctx, "ListFunctions", listFunctionsQuery, nil, &result); err != nil {
		return nil, err
	}

	seen := make(map[string]struct{})
	var functions []Function
	for _, et := range result.Events.Data {
		for _, fn := range et.Workflows {
			if _, ok := seen[fn.ID]; ok {
				continue
			}
			seen[fn.ID] = struct{}{}
			functions = append(functions, fn)
		}
	}

	if functions == nil {
		functions = []Function{}
	}
	return functions, nil
}

// GetFunction finds a function by slug. Since there is no root functionBySlug query,
// this lists all functions and filters client-side.
func (c *Client) GetFunction(ctx context.Context, slug string) (*Function, error) {
	functions, err := c.ListFunctions(ctx)
	if err != nil {
		return nil, err
	}
	for i := range functions {
		if functions[i].Slug == slug {
			return &functions[i], nil
		}
	}
	return nil, fmt.Errorf("inngest: function %q not found", slug)
}
