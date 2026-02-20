package xero

import (
	"fmt"
	"net/url"
)

// BankAccount represents a Xero bank account.
type BankAccount struct {
	AccountID         string `json:"AccountID"`
	Code              string `json:"Code"`
	Name              string `json:"Name"`
	Type              string `json:"Type"`
	BankAccountNumber string `json:"BankAccountNumber,omitempty"`
	CurrencyCode      string `json:"CurrencyCode,omitempty"`
}

// BankTransaction represents a Xero bank transaction.
type BankTransaction struct {
	BankTransactionID string  `json:"BankTransactionID"`
	Type              string  `json:"Type"`
	Status            string  `json:"Status"`
	Reference         string  `json:"Reference,omitempty"`
	Date              string  `json:"Date"`
	SubTotal          float64 `json:"SubTotal"`
	TotalTax          float64 `json:"TotalTax"`
	Total             float64 `json:"Total"`
	BankAccount       Account `json:"BankAccount"`
	Contact           Contact `json:"Contact,omitempty"`
	IsReconciled      bool    `json:"IsReconciled"`
	UpdatedDateUTC    string  `json:"UpdatedDateUTC,omitempty"`
}

type bankTransactionsResponse struct {
	BankTransactions []BankTransaction `json:"BankTransactions"`
}

// ListBankAccounts returns all bank accounts.
func (c *Client) ListBankAccounts() ([]BankAccount, error) {
	params := url.Values{"Type": {"BANK"}}
	var resp accountsResponse
	if err := c.get("/Accounts?"+params.Encode(), &resp); err != nil {
		return nil, err
	}
	banks := make([]BankAccount, len(resp.Accounts))
	for i, a := range resp.Accounts {
		banks[i] = BankAccount{
			AccountID: a.AccountID,
			Code:      a.Code,
			Name:      a.Name,
			Type:      a.Type,
		}
	}
	return banks, nil
}

// ListBankTransactions returns bank transactions, auto-paginating when page=0.
func (c *Client) ListBankTransactions(bankAccountID string, page int) ([]BankTransaction, error) {
	if page > 0 {
		return c.listBankTransactionsPage(bankAccountID, page)
	}
	var all []BankTransaction
	for p := 1; ; p++ {
		batch, err := c.listBankTransactionsPage(bankAccountID, p)
		if err != nil {
			return nil, err
		}
		all = append(all, batch...)
		if len(batch) < 100 {
			break
		}
	}
	return all, nil
}

func (c *Client) listBankTransactionsPage(bankAccountID string, page int) ([]BankTransaction, error) {
	params := url.Values{}
	if page > 0 {
		params.Set("page", fmt.Sprintf("%d", page))
	}
	if bankAccountID != "" {
		params.Set("BankAccountID", bankAccountID)
	}
	path := "/BankTransactions"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}
	var resp bankTransactionsResponse
	if err := c.get(path, &resp); err != nil {
		return nil, err
	}
	return resp.BankTransactions, nil
}

// GetBankTransaction fetches a single bank transaction by ID.
func (c *Client) GetBankTransaction(id string) (*BankTransaction, error) {
	var resp bankTransactionsResponse
	if err := c.get("/BankTransactions/"+url.PathEscape(id), &resp); err != nil {
		return nil, err
	}
	if len(resp.BankTransactions) == 0 {
		return nil, fmt.Errorf("bank transaction not found: %s", id)
	}
	return &resp.BankTransactions[0], nil
}
