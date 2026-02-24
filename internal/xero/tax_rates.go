package xero

// TaxRate represents a Xero tax rate.
type TaxRate struct {
	Name           string         `json:"Name"`
	TaxType        string         `json:"TaxType"`
	Status         string         `json:"Status"`
	ReportTaxType  string         `json:"ReportTaxType"`
	DisplayTaxRate float64        `json:"DisplayTaxRate"`
	EffectiveRate  float64        `json:"EffectiveRate"`
	TaxComponents  []TaxComponent `json:"TaxComponents,omitempty"`
}

// TaxComponent is a component of a tax rate.
type TaxComponent struct {
	Name       string  `json:"Name"`
	Rate       float64 `json:"Rate"`
	IsCompound bool    `json:"IsCompound"`
}

type taxRatesResponse struct {
	TaxRates []TaxRate `json:"TaxRates"`
}

// ListTaxRates fetches all tax rates.
func (c *Client) ListTaxRates() ([]TaxRate, error) {
	var resp taxRatesResponse
	if err := c.get("/TaxRates", &resp); err != nil {
		return nil, err
	}
	return resp.TaxRates, nil
}
