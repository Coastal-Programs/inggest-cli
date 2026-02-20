package xero

import (
	"fmt"
	"net/url"
)

// Account represents a Xero chart-of-accounts account.
type Account struct {
	AccountID               string  `json:"AccountID"`
	Code                    string  `json:"Code"`
	Name                    string  `json:"Name"`
	Type                    string  `json:"Type"`
	TaxType                 string  `json:"TaxType,omitempty"`
	Description             string  `json:"Description,omitempty"`
	Class                   string  `json:"Class"`
	Status                  string  `json:"Status"`
	EnablePaymentsToAccount bool    `json:"EnablePaymentsToAccount"`
	ShowInExpenseClaims     bool    `json:"ShowInExpenseClaims"`
}

type accountsResponse struct {
	Accounts []Account `json:"Accounts"`
}

// ListAccounts returns all accounts in the chart of accounts.
func (c *Client) ListAccounts(accountType, class string) ([]Account, error) {
	params := url.Values{}
	if accountType != "" {
		params.Set("Type", accountType)
	}
	path := "/Accounts"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}
	var resp accountsResponse
	if err := c.get(path, &resp); err != nil {
		return nil, err
	}
	if class == "" {
		return resp.Accounts, nil
	}
	// Filter by class client-side (Xero doesn't support this query param)
	filtered := make([]Account, 0)
	for _, a := range resp.Accounts {
		if a.Class == class {
			filtered = append(filtered, a)
		}
	}
	return filtered, nil
}

// GetAccount fetches a single account by ID or code.
func (c *Client) GetAccount(id string) (*Account, error) {
	var resp accountsResponse
	if err := c.get("/Accounts/"+url.PathEscape(id), &resp); err != nil {
		return nil, err
	}
	if len(resp.Accounts) == 0 {
		return nil, fmt.Errorf("account not found: %s", id)
	}
	return &resp.Accounts[0], nil
}
