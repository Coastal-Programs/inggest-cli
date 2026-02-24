package xero

import (
	"fmt"
	"net/url"
)

// Overpayment represents a Xero overpayment.
type Overpayment struct {
	OverpaymentID   string  `json:"OverpaymentID"`
	Type            string  `json:"Type"`
	Status          string  `json:"Status"`
	Contact         Contact `json:"Contact"`
	Total           float64 `json:"Total"`
	RemainingCredit float64 `json:"RemainingCredit"`
	DateString      string  `json:"DateString"`
}

// Prepayment represents a Xero prepayment.
type Prepayment struct {
	PrepaymentID    string  `json:"PrepaymentID"`
	Type            string  `json:"Type"`
	Status          string  `json:"Status"`
	Contact         Contact `json:"Contact"`
	Total           float64 `json:"Total"`
	RemainingCredit float64 `json:"RemainingCredit"`
	DateString      string  `json:"DateString"`
}

type overpaymentsResponse struct {
	Overpayments []Overpayment `json:"Overpayments"`
}

type prepaymentsResponse struct {
	Prepayments []Prepayment `json:"Prepayments"`
}

// ListOverpayments fetches overpayments.
// If page == 0, automatically paginates through all results.
func (c *Client) ListOverpayments(page int) ([]Overpayment, error) {
	if page > 0 {
		return c.listOverpaymentsPage(page)
	}
	var all []Overpayment
	for p := 1; ; p++ {
		batch, err := c.listOverpaymentsPage(p)
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

func (c *Client) listOverpaymentsPage(page int) ([]Overpayment, error) {
	params := url.Values{}
	params.Set("page", fmt.Sprintf("%d", page))
	path := "/Overpayments?" + params.Encode()
	var resp overpaymentsResponse
	if err := c.get(path, &resp); err != nil {
		return nil, err
	}
	return resp.Overpayments, nil
}

// ApplyOverpayment allocates an overpayment against an invoice.
func (c *Client) ApplyOverpayment(overpaymentID, invoiceID string, amount float64) (*Overpayment, error) {
	payload := map[string]any{
		"Allocations": []map[string]any{
			{
				"Invoice": map[string]string{"InvoiceID": invoiceID},
				"Amount":  amount,
			},
		},
	}
	var resp overpaymentsResponse
	if err := c.put("/Overpayments/"+url.PathEscape(overpaymentID)+"/Allocations", payload, &resp); err != nil {
		return nil, err
	}
	if len(resp.Overpayments) == 0 {
		return nil, fmt.Errorf("no overpayment returned from API")
	}
	return &resp.Overpayments[0], nil
}

// ListPrepayments fetches prepayments.
// If page == 0, automatically paginates through all results.
func (c *Client) ListPrepayments(page int) ([]Prepayment, error) {
	if page > 0 {
		return c.listPrepaymentsPage(page)
	}
	var all []Prepayment
	for p := 1; ; p++ {
		batch, err := c.listPrepaymentsPage(p)
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

func (c *Client) listPrepaymentsPage(page int) ([]Prepayment, error) {
	params := url.Values{}
	params.Set("page", fmt.Sprintf("%d", page))
	path := "/Prepayments?" + params.Encode()
	var resp prepaymentsResponse
	if err := c.get(path, &resp); err != nil {
		return nil, err
	}
	return resp.Prepayments, nil
}

// ApplyPrepayment allocates a prepayment against an invoice.
func (c *Client) ApplyPrepayment(prepaymentID, invoiceID string, amount float64) (*Prepayment, error) {
	payload := map[string]any{
		"Allocations": []map[string]any{
			{
				"Invoice": map[string]string{"InvoiceID": invoiceID},
				"Amount":  amount,
			},
		},
	}
	var resp prepaymentsResponse
	if err := c.put("/Prepayments/"+url.PathEscape(prepaymentID)+"/Allocations", payload, &resp); err != nil {
		return nil, err
	}
	if len(resp.Prepayments) == 0 {
		return nil, fmt.Errorf("no prepayment returned from API")
	}
	return &resp.Prepayments[0], nil
}
