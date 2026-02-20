package xero

import (
	"fmt"
	"net/url"
)

// ReportRow is a generic row in a report.
type ReportRow struct {
	RowType string      `json:"RowType"`
	Title   string      `json:"Title,omitempty"`
	Cells   []ReportCell `json:"Cells,omitempty"`
	Rows    []ReportRow  `json:"Rows,omitempty"`
}

// ReportCell is a cell in a report row.
type ReportCell struct {
	Value      string            `json:"Value"`
	Attributes []ReportAttribute `json:"Attributes,omitempty"`
}

// ReportAttribute is metadata on a cell.
type ReportAttribute struct {
	ID    string `json:"Id"`
	Value string `json:"Value"`
}

// Report is a Xero financial report.
type Report struct {
	ReportID    string      `json:"ReportID"`
	ReportName  string      `json:"ReportName"`
	ReportType  string      `json:"ReportType"`
	ReportTitles []string   `json:"ReportTitles"`
	ReportDate  string      `json:"ReportDate"`
	Rows        []ReportRow `json:"Rows"`
}

type reportsResponse struct {
	Reports []Report `json:"Reports"`
}

// GetProfitAndLoss fetches the P&L report.
func (c *Client) GetProfitAndLoss(fromDate, toDate string) (*Report, error) {
	return c.getReport("ProfitAndLoss", url.Values{
		"fromDate": {fromDate},
		"toDate":   {toDate},
	})
}

// GetBalanceSheet fetches the balance sheet report.
func (c *Client) GetBalanceSheet(date string) (*Report, error) {
	return c.getReport("BalanceSheet", url.Values{
		"date": {date},
	})
}

// GetTrialBalance fetches the trial balance report.
func (c *Client) GetTrialBalance(date string) (*Report, error) {
	return c.getReport("TrialBalance", url.Values{
		"date": {date},
	})
}

// GetAgedReceivables fetches the aged receivables report.
func (c *Client) GetAgedReceivables(date, contactID string) (*Report, error) {
	params := url.Values{"date": {date}}
	if contactID != "" {
		params.Set("contactId", contactID)
	}
	return c.getReport("AgedReceivablesByContact", params)
}

// GetAgedPayables fetches the aged payables report.
func (c *Client) GetAgedPayables(date, contactID string) (*Report, error) {
	params := url.Values{"date": {date}}
	if contactID != "" {
		params.Set("contactId", contactID)
	}
	return c.getReport("AgedPayablesByContact", params)
}

func (c *Client) getReport(name string, params url.Values) (*Report, error) {
	path := "/Reports/" + name
	if len(params) > 0 {
		path += "?" + params.Encode()
	}
	var resp reportsResponse
	if err := c.get(path, &resp); err != nil {
		return nil, err
	}
	if len(resp.Reports) == 0 {
		return nil, fmt.Errorf("no report returned for %s", name)
	}
	return &resp.Reports[0], nil
}
