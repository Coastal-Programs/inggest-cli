package xero

import (
	"fmt"
	"net/url"
)

// Budget represents a Xero budget.
type Budget struct {
	BudgetID    string       `json:"BudgetID"`
	Type        string       `json:"Type"`
	Description string       `json:"Description"`
	BudgetLines []BudgetLine `json:"BudgetLines,omitempty"`
}

// BudgetLine represents a line in a budget.
type BudgetLine struct {
	AccountID      string          `json:"AccountID"`
	AccountCode    string          `json:"AccountCode"`
	BudgetBalances []BudgetBalance `json:"BudgetBalances,omitempty"`
}

// BudgetBalance represents a budget balance for a period.
type BudgetBalance struct {
	Period string  `json:"Period"`
	Amount float64 `json:"Amount"`
	Notes  string  `json:"Notes,omitempty"`
}

type budgetsResponse struct {
	Budgets []Budget `json:"Budgets"`
}

// ListBudgets fetches all budgets.
func (c *Client) ListBudgets() ([]Budget, error) {
	var resp budgetsResponse
	if err := c.get("/Budgets", &resp); err != nil {
		return nil, err
	}
	return resp.Budgets, nil
}

// GetBudget fetches a single budget by ID.
func (c *Client) GetBudget(id string) (*Budget, error) {
	var resp budgetsResponse
	if err := c.get("/Budgets/"+url.PathEscape(id), &resp); err != nil {
		return nil, err
	}
	if len(resp.Budgets) == 0 {
		return nil, fmt.Errorf("budget not found: %s", id)
	}
	return &resp.Budgets[0], nil
}
