package xero

import (
	"fmt"
	"net/url"
	"time"
)

// Invoice represents a Xero invoice.
type Invoice struct {
	InvoiceID    string     `json:"InvoiceID"`
	InvoiceNumber string    `json:"InvoiceNumber"`
	Type         string     `json:"Type"`
	Status       string     `json:"Status"`
	Contact      Contact    `json:"Contact"`
	DateString   string     `json:"DateString"`
	DueDateString string    `json:"DueDateString"`
	LineAmountTypes string  `json:"LineAmountTypes"`
	LineItems    []LineItem `json:"LineItems"`
	SubTotal     float64    `json:"SubTotal"`
	TotalTax     float64    `json:"TotalTax"`
	Total        float64    `json:"Total"`
	AmountDue    float64    `json:"AmountDue"`
	AmountPaid   float64    `json:"AmountPaid"`
	CurrencyCode string     `json:"CurrencyCode"`
	UpdatedDateUTC string   `json:"UpdatedDateUTC"`
}

// LineItem is a line on an invoice or bill.
type LineItem struct {
	LineItemID  string  `json:"LineItemID,omitempty"`
	Description string  `json:"Description"`
	Quantity    float64 `json:"Quantity"`
	UnitAmount  float64 `json:"UnitAmount"`
	AccountCode string  `json:"AccountCode,omitempty"`
	TaxType     string  `json:"TaxType,omitempty"`
	LineAmount  float64 `json:"LineAmount,omitempty"`
}

type invoicesResponse struct {
	Invoices []Invoice `json:"Invoices"`
}

// InvoiceCreateInput is the payload for creating an invoice.
type InvoiceCreateInput struct {
	Type            string     `json:"Type"`
	Contact         ContactRef `json:"Contact"`
	Date            string     `json:"Date,omitempty"` // YYYY-MM-DD
	DueDate         string     `json:"DueDate,omitempty"`
	LineAmountTypes string     `json:"LineAmountTypes,omitempty"`
	LineItems       []LineItem `json:"LineItems"`
	CurrencyCode    string     `json:"CurrencyCode,omitempty"`
	Reference       string     `json:"Reference,omitempty"`
}

// ContactRef is a minimal reference to a contact.
type ContactRef struct {
	ContactID string `json:"ContactID,omitempty"`
	Name      string `json:"Name,omitempty"`
}

// ListInvoices fetches invoices with optional filtering.
// If page == 0, automatically paginates through all results.
func (c *Client) ListInvoices(status, invoiceType, dateFrom, dateTo string, page int) ([]Invoice, error) {
	if page > 0 {
		return c.listInvoicesPage(status, invoiceType, dateFrom, dateTo, page)
	}
	// Auto-paginate
	var all []Invoice
	for p := 1; ; p++ {
		batch, err := c.listInvoicesPage(status, invoiceType, dateFrom, dateTo, p)
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

func (c *Client) listInvoicesPage(status, invoiceType, dateFrom, dateTo string, page int) ([]Invoice, error) {
	params := url.Values{}
	if status != "" {
		params.Set("Statuses", status)
	}
	if invoiceType != "" {
		params.Set("Type", invoiceType)
	}
	if dateFrom != "" {
		params.Set("DateFrom", dateFrom)
	}
	if dateTo != "" {
		params.Set("DateTo", dateTo)
	}
	if page > 0 {
		params.Set("page", fmt.Sprintf("%d", page))
	}
	path := "/Invoices"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}
	var resp invoicesResponse
	if err := c.get(path, &resp); err != nil {
		return nil, err
	}
	return resp.Invoices, nil
}

// GetInvoice fetches a single invoice by ID or number.
func (c *Client) GetInvoice(id string) (*Invoice, error) {
	var resp invoicesResponse
	if err := c.get("/Invoices/"+id, &resp); err != nil {
		return nil, err
	}
	if len(resp.Invoices) == 0 {
		return nil, fmt.Errorf("invoice not found: %s", id)
	}
	return &resp.Invoices[0], nil
}

// CreateInvoice creates a new invoice or bill.
func (c *Client) CreateInvoice(input InvoiceCreateInput) (*Invoice, error) {
	if input.Date == "" {
		input.Date = time.Now().Format("2006-01-02")
	}
	if input.LineAmountTypes == "" {
		input.LineAmountTypes = "EXCLUSIVE"
	}
	payload := map[string]any{"Invoices": []InvoiceCreateInput{input}}
	var resp invoicesResponse
	if err := c.put("/Invoices", payload, &resp); err != nil {
		return nil, err
	}
	if len(resp.Invoices) == 0 {
		return nil, fmt.Errorf("no invoice returned from API")
	}
	return &resp.Invoices[0], nil
}

// VoidInvoice sets an invoice's status to VOIDED.
func (c *Client) VoidInvoice(id string) (*Invoice, error) {
	payload := map[string]any{
		"Invoices": []map[string]string{
			{"InvoiceID": id, "Status": "VOIDED"},
		},
	}
	var resp invoicesResponse
	if err := c.post("/Invoices/"+id, payload, &resp); err != nil {
		return nil, err
	}
	if len(resp.Invoices) == 0 {
		return nil, fmt.Errorf("no invoice returned from API")
	}
	return &resp.Invoices[0], nil
}

// EmailInvoice sends an invoice to the contact's email.
func (c *Client) EmailInvoice(id string) error {
	return c.post("/Invoices/"+id+"/Email", nil, nil)
}
