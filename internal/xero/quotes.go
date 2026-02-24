package xero

import (
	"fmt"
	"net/url"
)

// Quote represents a Xero quote.
type Quote struct {
	QuoteID          string     `json:"QuoteID"`
	QuoteNumber      string     `json:"QuoteNumber"`
	Status           string     `json:"Status"`
	Contact          Contact    `json:"Contact"`
	LineItems        []LineItem `json:"LineItems"`
	SubTotal         float64    `json:"SubTotal"`
	TotalTax         float64    `json:"TotalTax"`
	Total            float64    `json:"Total"`
	ExpiryDateString string     `json:"ExpiryDateString"`
	DateString       string     `json:"DateString"`
}

// QuoteCreateInput is the payload for creating a quote.
type QuoteCreateInput struct {
	Contact    ContactRef `json:"Contact"`
	ExpiryDate string     `json:"ExpiryDate,omitempty"`
	LineItems  []LineItem `json:"LineItems"`
}

type quotesResponse struct {
	Quotes []Quote `json:"Quotes"`
}

// ListQuotes fetches quotes with optional status filtering.
// If page == 0, automatically paginates through all results.
func (c *Client) ListQuotes(status string, page int) ([]Quote, error) {
	if page > 0 {
		return c.listQuotesPage(status, page)
	}
	var all []Quote
	for p := 1; ; p++ {
		batch, err := c.listQuotesPage(status, p)
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

func (c *Client) listQuotesPage(status string, page int) ([]Quote, error) {
	params := url.Values{}
	if status != "" {
		params.Set("Status", status)
	}
	params.Set("page", fmt.Sprintf("%d", page))
	path := "/Quotes?" + params.Encode()
	var resp quotesResponse
	if err := c.get(path, &resp); err != nil {
		return nil, err
	}
	return resp.Quotes, nil
}

// GetQuote fetches a single quote by ID.
func (c *Client) GetQuote(id string) (*Quote, error) {
	var resp quotesResponse
	if err := c.get("/Quotes/"+url.PathEscape(id), &resp); err != nil {
		return nil, err
	}
	if len(resp.Quotes) == 0 {
		return nil, fmt.Errorf("quote not found: %s", id)
	}
	return &resp.Quotes[0], nil
}

// CreateQuote creates a new quote.
func (c *Client) CreateQuote(input QuoteCreateInput) (*Quote, error) {
	payload := map[string]any{"Quotes": []QuoteCreateInput{input}}
	var resp quotesResponse
	if err := c.put("/Quotes", payload, &resp); err != nil {
		return nil, err
	}
	if len(resp.Quotes) == 0 {
		return nil, fmt.Errorf("no quote returned from API")
	}
	return &resp.Quotes[0], nil
}

// ConvertQuoteToInvoice creates an ACCREC invoice from an existing quote's contact and line items.
// The Xero Accounting API has no native convert endpoint; this composes GetQuote + CreateInvoice.
func (c *Client) ConvertQuoteToInvoice(id string) (*Invoice, error) {
	quote, err := c.GetQuote(id)
	if err != nil {
		return nil, fmt.Errorf("fetching quote: %w", err)
	}
	input := InvoiceCreateInput{
		Type:      "ACCREC",
		Contact:   ContactRef{ContactID: quote.Contact.ContactID},
		LineItems: quote.LineItems,
	}
	return c.CreateInvoice(input)
}
