package xero

import (
	"fmt"
	"net/url"
	"time"
)

// Payment represents a Xero payment.
type Payment struct {
	PaymentID    string  `json:"PaymentID"`
	Date         string  `json:"Date"`
	Amount       float64 `json:"Amount"`
	Reference    string  `json:"Reference,omitempty"`
	PaymentType  string  `json:"PaymentType"`
	Status       string  `json:"Status"`
	Invoice      Invoice `json:"Invoice,omitempty"`
	Account      Account `json:"Account,omitempty"`
	CurrencyRate float64 `json:"CurrencyRate,omitempty"`
	UpdatedDateUTC string `json:"UpdatedDateUTC,omitempty"`
}

type paymentsResponse struct {
	Payments []Payment `json:"Payments"`
}

// PaymentCreateInput is the payload for applying a payment to an invoice.
type PaymentCreateInput struct {
	Invoice   InvoiceRef `json:"Invoice"`
	Account   AccountRef `json:"Account"`
	Date      string     `json:"Date"`
	Amount    float64    `json:"Amount"`
	Reference string     `json:"Reference,omitempty"`
}

// InvoiceRef is a minimal reference to an invoice.
type InvoiceRef struct {
	InvoiceID string `json:"InvoiceID,omitempty"`
}

// AccountRef is a minimal reference to an account.
type AccountRef struct {
	AccountID string `json:"AccountID,omitempty"`
	Code      string `json:"Code,omitempty"`
}

// ListPayments fetches payments with optional filtering.
func (c *Client) ListPayments(status string, page int) ([]Payment, error) {
	params := url.Values{}
	if status != "" {
		params.Set("Statuses", status)
	}
	if page > 0 {
		params.Set("page", fmt.Sprintf("%d", page))
	}
	path := "/Payments"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}
	var resp paymentsResponse
	if err := c.get(path, &resp); err != nil {
		return nil, err
	}
	return resp.Payments, nil
}

// GetPayment fetches a single payment by ID.
func (c *Client) GetPayment(id string) (*Payment, error) {
	var resp paymentsResponse
	if err := c.get("/Payments/"+id, &resp); err != nil {
		return nil, err
	}
	if len(resp.Payments) == 0 {
		return nil, fmt.Errorf("payment not found: %s", id)
	}
	return &resp.Payments[0], nil
}

// CreatePayment applies a payment to an invoice.
func (c *Client) CreatePayment(input PaymentCreateInput) (*Payment, error) {
	if input.Date == "" {
		input.Date = time.Now().Format("2006-01-02")
	}
	var resp paymentsResponse
	if err := c.post("/Payments", input, &resp); err != nil {
		return nil, err
	}
	if len(resp.Payments) == 0 {
		return nil, fmt.Errorf("no payment returned from API")
	}
	return &resp.Payments[0], nil
}
