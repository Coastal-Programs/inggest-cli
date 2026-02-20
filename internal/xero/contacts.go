package xero

import (
	"fmt"
	"net/url"
)

// Contact represents a Xero contact (customer or supplier).
type Contact struct {
	ContactID       string   `json:"ContactID"`
	Name            string   `json:"Name"`
	FirstName       string   `json:"FirstName,omitempty"`
	LastName        string   `json:"LastName,omitempty"`
	EmailAddress    string   `json:"EmailAddress,omitempty"`
	IsCustomer      bool     `json:"IsCustomer"`
	IsSupplier      bool     `json:"IsSupplier"`
	AccountNumber   string   `json:"AccountNumber,omitempty"`
	TaxNumber       string   `json:"TaxNumber,omitempty"`
	ContactStatus   string   `json:"ContactStatus"`
	UpdatedDateUTC  string   `json:"UpdatedDateUTC,omitempty"`
}

type contactsResponse struct {
	Contacts []Contact `json:"Contacts"`
}

// ContactCreateInput is the payload for creating or updating a contact.
type ContactCreateInput struct {
	Name         string `json:"Name"`
	FirstName    string `json:"FirstName,omitempty"`
	LastName     string `json:"LastName,omitempty"`
	EmailAddress string `json:"EmailAddress,omitempty"`
	AccountNumber string `json:"AccountNumber,omitempty"`
	TaxNumber    string `json:"TaxNumber,omitempty"`
	IsCustomer   bool   `json:"IsCustomer,omitempty"`
	IsSupplier   bool   `json:"IsSupplier,omitempty"`
}

// ListContacts fetches contacts with optional search/filter.
// If page == 0, automatically paginates through all results.
func (c *Client) ListContacts(search string, isCustomer, isSupplier bool, page int) ([]Contact, error) {
	if page > 0 {
		return c.listContactsPage(search, isCustomer, isSupplier, page)
	}
	var all []Contact
	for p := 1; ; p++ {
		batch, err := c.listContactsPage(search, isCustomer, isSupplier, p)
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

func (c *Client) listContactsPage(search string, isCustomer, isSupplier bool, page int) ([]Contact, error) {
	params := url.Values{}
	if search != "" {
		params.Set("searchTerm", search)
	}
	if isCustomer {
		params.Set("IsCustomer", "true")
	}
	if isSupplier {
		params.Set("IsSupplier", "true")
	}
	if page > 0 {
		params.Set("page", fmt.Sprintf("%d", page))
	}
	path := "/Contacts"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}
	var resp contactsResponse
	if err := c.get(path, &resp); err != nil {
		return nil, err
	}
	return resp.Contacts, nil
}

// GetContact fetches a single contact by ID.
func (c *Client) GetContact(id string) (*Contact, error) {
	var resp contactsResponse
	if err := c.get("/Contacts/"+id, &resp); err != nil {
		return nil, err
	}
	if len(resp.Contacts) == 0 {
		return nil, fmt.Errorf("contact not found: %s", id)
	}
	return &resp.Contacts[0], nil
}

// CreateContact creates a new contact.
func (c *Client) CreateContact(input ContactCreateInput) (*Contact, error) {
	payload := map[string]any{"Contacts": []ContactCreateInput{input}}
	var resp contactsResponse
	if err := c.put("/Contacts", payload, &resp); err != nil {
		return nil, err
	}
	if len(resp.Contacts) == 0 {
		return nil, fmt.Errorf("no contact returned from API")
	}
	return &resp.Contacts[0], nil
}

// UpdateContact updates an existing contact by ID.
func (c *Client) UpdateContact(id string, input ContactCreateInput) (*Contact, error) {
	payload := map[string]any{"Contacts": []ContactCreateInput{input}}
	var resp contactsResponse
	if err := c.post("/Contacts/"+id, payload, &resp); err != nil {
		return nil, err
	}
	if len(resp.Contacts) == 0 {
		return nil, fmt.Errorf("no contact returned from API")
	}
	return &resp.Contacts[0], nil
}
