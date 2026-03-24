package inngest

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestListEnvironments(t *testing.T) {
	response := `{
		"data": {
			"envs": {
				"edges": [
					{
						"node": {
							"id": "env-1",
							"name": "Production",
							"slug": "production",
							"type": "production",
							"isAutoArchiveEnabled": true,
							"createdAt": "2024-01-15T10:00:00Z"
						}
					},
					{
						"node": {
							"id": "env-2",
							"name": "Staging",
							"slug": "staging",
							"type": "branch",
							"isAutoArchiveEnabled": false,
							"createdAt": "2024-02-01T12:00:00Z"
						}
					}
				],
				"pageInfo": {
					"hasNextPage": false,
					"endCursor": ""
				}
			}
		}
	}`

	var captured graphqlRequest
	srv := newTestServer(t, response, &captured)
	defer srv.Close()

	client := NewClient(ClientOptions{
		SigningKey: "test-key",
		APIBaseURL: srv.URL,
	})

	envs, err := client.ListEnvironments(context.Background())
	if err != nil {
		t.Fatalf("ListEnvironments returned error: %v", err)
	}

	if !strings.Contains(captured.Query, "envs") {
		t.Errorf("expected query to contain 'envs', got: %s", captured.Query)
	}

	if len(envs) != 2 {
		t.Fatalf("expected 2 environments, got %d", len(envs))
	}

	// First env.
	if envs[0].ID != "env-1" {
		t.Errorf("expected first env ID 'env-1', got %q", envs[0].ID)
	}
	if envs[0].Name != "Production" {
		t.Errorf("expected Name 'Production', got %q", envs[0].Name)
	}
	if envs[0].Slug != "production" {
		t.Errorf("expected Slug 'production', got %q", envs[0].Slug)
	}
	if envs[0].Type != "production" {
		t.Errorf("expected Type 'production', got %q", envs[0].Type)
	}
	if !envs[0].IsAutoArchiveEnabled {
		t.Error("expected IsAutoArchiveEnabled true, got false")
	}

	// Second env.
	if envs[1].ID != "env-2" {
		t.Errorf("expected second env ID 'env-2', got %q", envs[1].ID)
	}
	if envs[1].Name != "Staging" {
		t.Errorf("expected Name 'Staging', got %q", envs[1].Name)
	}
	if envs[1].Type != "branch" {
		t.Errorf("expected Type 'branch', got %q", envs[1].Type)
	}
}

func TestListEnvironments_Error(t *testing.T) {
	response := `{"data": null, "errors": [{"message": "unauthorized"}]}`

	srv := newTestServer(t, response, nil)
	defer srv.Close()

	client := NewClient(ClientOptions{
		SigningKey: "test-key",
		APIBaseURL: srv.URL,
	})

	envs, err := client.ListEnvironments(context.Background())
	if err == nil {
		t.Fatal("expected error for GraphQL error response, got nil")
	}
	if envs != nil {
		t.Errorf("expected nil envs, got %+v", envs)
	}
	// "unauthorized" triggers the account-level auth error path.
	if !errors.Is(err, ErrAccountAuthRequired) {
		t.Errorf("expected ErrAccountAuthRequired sentinel, got: %v", err)
	}
}

func TestListEnvironments_UnauthenticatedHint(t *testing.T) {
	response := `{"data": null, "errors": [{"message": "UNAUTHENTICATED"}]}`

	srv := newTestServer(t, response, nil)
	defer srv.Close()

	client := NewClient(ClientOptions{
		SigningKey: "test-key",
		APIBaseURL: srv.URL,
	})

	_, err := client.ListEnvironments(context.Background())
	if err == nil {
		t.Fatal("expected error for UNAUTHENTICATED response, got nil")
	}
	if !errors.Is(err, ErrAccountAuthRequired) {
		t.Errorf("expected ErrAccountAuthRequired sentinel, got: %v", err)
	}
	if !strings.Contains(err.Error(), "account-level") {
		t.Errorf("expected error to contain auth hint, got: %v", err)
	}
	if !strings.Contains(err.Error(), "https://app.inngest.com/env") {
		t.Errorf("expected error to contain dashboard link, got: %v", err)
	}
}

func TestListEnvironments_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("server error"))
	}))
	defer srv.Close()

	client := NewClient(ClientOptions{
		SigningKey: "test-key",
		APIBaseURL: srv.URL,
	})

	envs, err := client.ListEnvironments(context.Background())
	if err == nil {
		t.Fatal("expected error for HTTP 500, got nil")
	}
	if envs != nil {
		t.Errorf("expected nil envs, got %+v", envs)
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected error to contain '500', got: %v", err)
	}
}

func TestGetEnvironment(t *testing.T) {
	response := `{
		"data": {
			"envs": {
				"edges": [
					{
						"node": {
							"id": "env-1",
							"name": "Production",
							"slug": "production",
							"type": "production",
							"createdAt": "2024-01-15T10:00:00Z"
						}
					},
					{
						"node": {
							"id": "env-2",
							"name": "Staging",
							"slug": "staging",
							"type": "branch",
							"createdAt": "2024-02-01T12:00:00Z"
						}
					}
				],
				"pageInfo": {
					"hasNextPage": false
				}
			}
		}
	}`

	srv := newTestServer(t, response, nil)
	defer srv.Close()

	client := NewClient(ClientOptions{
		SigningKey: "test-key",
		APIBaseURL: srv.URL,
	})

	// Find by name.
	env, err := client.GetEnvironment(context.Background(), "Production")
	if err != nil {
		t.Fatalf("GetEnvironment returned error: %v", err)
	}
	if env.ID != "env-1" {
		t.Errorf("expected ID 'env-1', got %q", env.ID)
	}

	// Find by slug.
	env, err = client.GetEnvironment(context.Background(), "staging")
	if err != nil {
		t.Fatalf("GetEnvironment by slug returned error: %v", err)
	}
	if env.ID != "env-2" {
		t.Errorf("expected ID 'env-2', got %q", env.ID)
	}

	// Find by ID.
	env, err = client.GetEnvironment(context.Background(), "env-1")
	if err != nil {
		t.Fatalf("GetEnvironment by ID returned error: %v", err)
	}
	if env.Name != "Production" {
		t.Errorf("expected Name 'Production', got %q", env.Name)
	}
}

func TestGetEnvironment_NotFound(t *testing.T) {
	response := `{
		"data": {
			"envs": {
				"edges": [],
				"pageInfo": {"hasNextPage": false}
			}
		}
	}`

	srv := newTestServer(t, response, nil)
	defer srv.Close()

	client := NewClient(ClientOptions{
		SigningKey: "test-key",
		APIBaseURL: srv.URL,
	})

	env, err := client.GetEnvironment(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for missing env, got nil")
	}
	if env != nil {
		t.Errorf("expected nil env, got %+v", env)
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected error to contain 'not found', got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("expected error to contain 'nonexistent', got %q", err.Error())
	}
}

func TestGetEnvironment_GraphQLError(t *testing.T) {
	response := `{"data": null, "errors": [{"message": "internal server error"}]}`

	srv := newTestServer(t, response, nil)
	defer srv.Close()

	client := NewClient(ClientOptions{
		SigningKey: "test-key",
		APIBaseURL: srv.URL,
	})

	env, err := client.GetEnvironment(context.Background(), "env-1")
	if err == nil {
		t.Fatal("expected error for GraphQL error response, got nil")
	}
	if env != nil {
		t.Errorf("expected nil env, got %+v", env)
	}
	if !strings.Contains(err.Error(), "internal server error") {
		t.Errorf("expected error to contain 'internal server error', got: %v", err)
	}
}
