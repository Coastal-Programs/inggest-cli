package xero

import "fmt"

// Item represents a Xero inventory item.
type Item struct {
	ItemID      string  `json:"ItemID"`
	Code        string  `json:"Code"`
	Name        string  `json:"Name"`
	Description string  `json:"Description,omitempty"`
	IsTracked   bool    `json:"IsTracked"`
	IsSold      bool    `json:"IsSold"`
	IsPurchased bool    `json:"IsPurchased"`
	SalesDetails    ItemDetails `json:"SalesDetails,omitempty"`
	PurchaseDetails ItemDetails `json:"PurchaseDetails,omitempty"`
	UpdatedDateUTC  string      `json:"UpdatedDateUTC,omitempty"`
}

// ItemDetails holds pricing and account info for an item.
type ItemDetails struct {
	UnitPrice   float64 `json:"UnitPrice,omitempty"`
	AccountCode string  `json:"AccountCode,omitempty"`
	TaxType     string  `json:"TaxType,omitempty"`
}

type itemsResponse struct {
	Items []Item `json:"Items"`
}

// ItemCreateInput is the payload for creating an item.
type ItemCreateInput struct {
	Code        string      `json:"Code"`
	Name        string      `json:"Name"`
	Description string      `json:"Description,omitempty"`
	IsSold      bool        `json:"IsSold,omitempty"`
	IsPurchased bool        `json:"IsPurchased,omitempty"`
	SalesDetails    ItemDetails `json:"SalesDetails,omitempty"`
	PurchaseDetails ItemDetails `json:"PurchaseDetails,omitempty"`
}

// ListItems returns all inventory items.
func (c *Client) ListItems() ([]Item, error) {
	var resp itemsResponse
	if err := c.get("/Items", &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// GetItem fetches a single item by ID or code.
func (c *Client) GetItem(id string) (*Item, error) {
	var resp itemsResponse
	if err := c.get("/Items/"+id, &resp); err != nil {
		return nil, err
	}
	if len(resp.Items) == 0 {
		return nil, fmt.Errorf("item not found: %s", id)
	}
	return &resp.Items[0], nil
}

// CreateItem creates a new inventory item.
func (c *Client) CreateItem(input ItemCreateInput) (*Item, error) {
	payload := map[string]any{"Items": []ItemCreateInput{input}}
	var resp itemsResponse
	if err := c.put("/Items", payload, &resp); err != nil {
		return nil, err
	}
	if len(resp.Items) == 0 {
		return nil, fmt.Errorf("no item returned from API")
	}
	return &resp.Items[0], nil
}
