package xero

import (
	"fmt"
	"net/url"
)

// ReportRow is a generic row in a report.
type ReportRow struct {
	RowType string       `json:"RowType"`
	Title   string       `json:"Title,omitempty"`
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
	ReportID     string      `json:"ReportID"`
	ReportName   string      `json:"ReportName"`
	ReportType   string      `json:"ReportType"`
	ReportTitles []string    `json:"ReportTitles"`
	ReportDate   string      `json:"ReportDate"`
	Rows         []ReportRow `json:"Rows"`
}

type reportsResponse struct {
	Reports []Report `json:"Reports"`
}

// GetProfitAndLoss fetches the P&L report, optionally filtered by tracking category/option.
func (c *Client) GetProfitAndLoss(fromDate, toDate, trackingCategoryID, trackingOptionID string) (*Report, error) {
	params := url.Values{
		"fromDate": {fromDate},
		"toDate":   {toDate},
	}
	if trackingCategoryID != "" {
		params.Set("trackingCategoryID", trackingCategoryID)
	}
	if trackingOptionID != "" {
		params.Set("trackingOptionID", trackingOptionID)
	}
	return c.getReport("ProfitAndLoss", params)
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

// GetCashFlow fetches the cash flow summary report.
func (c *Client) GetCashFlow(fromDate, toDate string) (*Report, error) {
	return c.getReport("CashSummary", url.Values{
		"fromDate": {fromDate},
		"toDate":   {toDate},
	})
}

// GetBudgetVariance fetches the budget variance report.
func (c *Client) GetBudgetVariance(budgetID, fromDate, toDate string) (*Report, error) {
	params := url.Values{
		"fromDate": {fromDate},
		"toDate":   {toDate},
	}
	if budgetID != "" {
		params.Set("budgetID", budgetID)
	}
	return c.getReport("BudgetVariance", params)
}

// GetAccountTransactions fetches the account transactions report.
func (c *Client) GetAccountTransactions(accountCode, fromDate, toDate string) (*Report, error) {
	return c.getReport("AccountTransactions", url.Values{
		"accountCode": {accountCode},
		"fromDate":    {fromDate},
		"toDate":      {toDate},
	})
}

// GetExecutiveSummary fetches the executive summary report.
func (c *Client) GetExecutiveSummary(fromDate, toDate string) (*Report, error) {
	return c.getReport("ExecutiveSummary", url.Values{
		"fromDate": {fromDate},
		"toDate":   {toDate},
	})
}

// GetBankSummary fetches the bank summary report.
func (c *Client) GetBankSummary(fromDate, toDate string) (*Report, error) {
	return c.getReport("BankSummary", url.Values{
		"fromDate": {fromDate},
		"toDate":   {toDate},
	})
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
