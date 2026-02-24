package xero

import (
	"fmt"
	"net/url"
)

// TrackingCategory represents a Xero tracking category.
type TrackingCategory struct {
	TrackingCategoryID string           `json:"TrackingCategoryID"`
	Name               string           `json:"Name"`
	Status             string           `json:"Status"`
	Options            []TrackingOption `json:"Options,omitempty"`
}

// TrackingOption represents an option within a tracking category.
type TrackingOption struct {
	TrackingOptionID string `json:"TrackingOptionID"`
	Name             string `json:"Name"`
	Status           string `json:"Status"`
}

type trackingCategoriesResponse struct {
	TrackingCategories []TrackingCategory `json:"TrackingCategories"`
}

type trackingOptionsResponse struct {
	Options []TrackingOption `json:"Options"`
}

// ListTrackingCategories fetches all tracking categories.
func (c *Client) ListTrackingCategories() ([]TrackingCategory, error) {
	var resp trackingCategoriesResponse
	if err := c.get("/TrackingCategories", &resp); err != nil {
		return nil, err
	}
	return resp.TrackingCategories, nil
}

// GetTrackingCategory fetches a single tracking category by ID.
func (c *Client) GetTrackingCategory(id string) (*TrackingCategory, error) {
	var resp trackingCategoriesResponse
	if err := c.get("/TrackingCategories/"+url.PathEscape(id), &resp); err != nil {
		return nil, err
	}
	if len(resp.TrackingCategories) == 0 {
		return nil, fmt.Errorf("tracking category not found: %s", id)
	}
	return &resp.TrackingCategories[0], nil
}

// CreateTrackingCategory creates a new tracking category.
func (c *Client) CreateTrackingCategory(name string) (*TrackingCategory, error) {
	payload := map[string]any{"TrackingCategory": map[string]string{"Name": name}}
	var resp trackingCategoriesResponse
	if err := c.put("/TrackingCategories", payload, &resp); err != nil {
		return nil, err
	}
	if len(resp.TrackingCategories) == 0 {
		return nil, fmt.Errorf("no tracking category returned from API")
	}
	return &resp.TrackingCategories[0], nil
}

// AddTrackingOption adds an option to a tracking category.
func (c *Client) AddTrackingOption(categoryID, name string) (*TrackingOption, error) {
	payload := map[string]any{"TrackingOption": map[string]string{"Name": name}}
	var resp trackingOptionsResponse
	if err := c.put("/TrackingCategories/"+url.PathEscape(categoryID)+"/Options", payload, &resp); err != nil {
		return nil, err
	}
	if len(resp.Options) == 0 {
		return nil, fmt.Errorf("no tracking option returned from API")
	}
	return &resp.Options[0], nil
}
