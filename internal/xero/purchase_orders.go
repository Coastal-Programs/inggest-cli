package xero

import (
	"fmt"
	"net/url"
)

// PurchaseOrder represents a Xero purchase order.
type PurchaseOrder struct {
	PurchaseOrderID     string     `json:"PurchaseOrderID"`
	PurchaseOrderNumber string     `json:"PurchaseOrderNumber"`
	Status              string     `json:"Status"`
	Contact             Contact    `json:"Contact"`
	LineItems           []LineItem `json:"LineItems"`
	SubTotal            float64    `json:"SubTotal"`
	TotalTax            float64    `json:"TotalTax"`
	Total               float64    `json:"Total"`
	DateString          string     `json:"DateString"`
	DeliveryDateString  string     `json:"DeliveryDateString"`
}

// PurchaseOrderCreateInput is the payload for creating a purchase order.
type PurchaseOrderCreateInput struct {
	Contact      ContactRef `json:"Contact"`
	DeliveryDate string     `json:"DeliveryDate,omitempty"`
	LineItems    []LineItem `json:"LineItems"`
}

type purchaseOrdersResponse struct {
	PurchaseOrders []PurchaseOrder `json:"PurchaseOrders"`
}

// ListPurchaseOrders fetches purchase orders with optional status filtering.
// If page == 0, automatically paginates through all results.
func (c *Client) ListPurchaseOrders(status string, page int) ([]PurchaseOrder, error) {
	if page > 0 {
		return c.listPurchaseOrdersPage(status, page)
	}
	var all []PurchaseOrder
	for p := 1; ; p++ {
		batch, err := c.listPurchaseOrdersPage(status, p)
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

func (c *Client) listPurchaseOrdersPage(status string, page int) ([]PurchaseOrder, error) {
	params := url.Values{}
	if status != "" {
		params.Set("Status", status)
	}
	params.Set("page", fmt.Sprintf("%d", page))
	path := "/PurchaseOrders?" + params.Encode()
	var resp purchaseOrdersResponse
	if err := c.get(path, &resp); err != nil {
		return nil, err
	}
	return resp.PurchaseOrders, nil
}

// GetPurchaseOrder fetches a single purchase order by ID.
func (c *Client) GetPurchaseOrder(id string) (*PurchaseOrder, error) {
	var resp purchaseOrdersResponse
	if err := c.get("/PurchaseOrders/"+url.PathEscape(id), &resp); err != nil {
		return nil, err
	}
	if len(resp.PurchaseOrders) == 0 {
		return nil, fmt.Errorf("purchase order not found: %s", id)
	}
	return &resp.PurchaseOrders[0], nil
}

// CreatePurchaseOrder creates a new purchase order.
func (c *Client) CreatePurchaseOrder(input PurchaseOrderCreateInput) (*PurchaseOrder, error) {
	payload := map[string]any{"PurchaseOrders": []PurchaseOrderCreateInput{input}}
	var resp purchaseOrdersResponse
	if err := c.put("/PurchaseOrders", payload, &resp); err != nil {
		return nil, err
	}
	if len(resp.PurchaseOrders) == 0 {
		return nil, fmt.Errorf("no purchase order returned from API")
	}
	return &resp.PurchaseOrders[0], nil
}
