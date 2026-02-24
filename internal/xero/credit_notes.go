package xero

import (
	"fmt"
	"net/url"
	"time"
)

// CreditNote represents a Xero credit note.
type CreditNote struct {
	CreditNoteID     string                 `json:"CreditNoteID"`
	CreditNoteNumber string                 `json:"CreditNoteNumber"`
	Type             string                 `json:"Type"`
	Status           string                 `json:"Status"`
	Contact          Contact                `json:"Contact"`
	LineItems        []LineItem             `json:"LineItems"`
	SubTotal         float64                `json:"SubTotal"`
	TotalTax         float64                `json:"TotalTax"`
	Total            float64                `json:"Total"`
	RemainingCredit  float64                `json:"RemainingCredit"`
	Allocations      []CreditNoteAllocation `json:"Allocations,omitempty"`
	DateString       string                 `json:"DateString"`
	DueDateString    string                 `json:"DueDateString"`
	CurrencyCode     string                 `json:"CurrencyCode"`
	UpdatedDateUTC   string                 `json:"UpdatedDateUTC"`
}

// CreditNoteAllocation represents an allocation of a credit note against an invoice.
type CreditNoteAllocation struct {
	AllocationID string     `json:"AllocationID"`
	Invoice      InvoiceRef `json:"Invoice"`
	Amount       float64    `json:"Amount"`
	Date         string     `json:"Date"`
}

// CreditNoteCreateInput is the payload for creating a credit note.
type CreditNoteCreateInput struct {
	Type      string     `json:"Type"`
	Contact   ContactRef `json:"Contact"`
	Date      string     `json:"Date,omitempty"`
	DueDate   string     `json:"DueDate,omitempty"`
	LineItems []LineItem `json:"LineItems"`
}

type creditNotesResponse struct {
	CreditNotes []CreditNote `json:"CreditNotes"`
}

// ListCreditNotes fetches credit notes with optional filtering.
// If page == 0, automatically paginates through all results.
func (c *Client) ListCreditNotes(status, cnType string, page int) ([]CreditNote, error) {
	if page > 0 {
		return c.listCreditNotesPage(status, cnType, page)
	}
	var all []CreditNote
	for p := 1; ; p++ {
		batch, err := c.listCreditNotesPage(status, cnType, p)
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

func (c *Client) listCreditNotesPage(status, cnType string, page int) ([]CreditNote, error) {
	params := url.Values{}
	if status != "" {
		params.Set("Statuses", status)
	}
	if cnType != "" {
		params.Set("Type", cnType)
	}
	params.Set("page", fmt.Sprintf("%d", page))
	path := "/CreditNotes?" + params.Encode()
	var resp creditNotesResponse
	if err := c.get(path, &resp); err != nil {
		return nil, err
	}
	return resp.CreditNotes, nil
}

// GetCreditNote fetches a single credit note by ID.
func (c *Client) GetCreditNote(id string) (*CreditNote, error) {
	var resp creditNotesResponse
	if err := c.get("/CreditNotes/"+url.PathEscape(id), &resp); err != nil {
		return nil, err
	}
	if len(resp.CreditNotes) == 0 {
		return nil, fmt.Errorf("credit note not found: %s", id)
	}
	return &resp.CreditNotes[0], nil
}

// CreateCreditNote creates a new credit note.
func (c *Client) CreateCreditNote(input CreditNoteCreateInput) (*CreditNote, error) {
	if input.Date == "" {
		input.Date = time.Now().Format("2006-01-02")
	}
	payload := map[string]any{"CreditNotes": []CreditNoteCreateInput{input}}
	var resp creditNotesResponse
	if err := c.put("/CreditNotes", payload, &resp); err != nil {
		return nil, err
	}
	if len(resp.CreditNotes) == 0 {
		return nil, fmt.Errorf("no credit note returned from API")
	}
	return &resp.CreditNotes[0], nil
}

// ApplyCreditNote allocates a credit note against an invoice.
func (c *Client) ApplyCreditNote(creditNoteID, invoiceID string, amount float64) (*CreditNote, error) {
	payload := map[string]any{
		"Allocations": []map[string]any{
			{
				"Invoice": map[string]string{"InvoiceID": invoiceID},
				"Amount":  amount,
			},
		},
	}
	var resp creditNotesResponse
	if err := c.put("/CreditNotes/"+url.PathEscape(creditNoteID)+"/Allocations", payload, &resp); err != nil {
		return nil, err
	}
	if len(resp.CreditNotes) == 0 {
		return nil, fmt.Errorf("no credit note returned from API")
	}
	return &resp.CreditNotes[0], nil
}
