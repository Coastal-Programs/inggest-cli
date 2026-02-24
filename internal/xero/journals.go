package xero

import (
	"fmt"
	"net/url"
	"time"
)

// ManualJournal represents a Xero manual journal.
type ManualJournal struct {
	ManualJournalID string              `json:"ManualJournalID"`
	Narration       string              `json:"Narration"`
	Date            string              `json:"Date"`
	Status          string              `json:"Status"`
	JournalLines    []ManualJournalLine `json:"JournalLines"`
}

// ManualJournalLine is a line in a manual journal.
type ManualJournalLine struct {
	AccountCode string  `json:"AccountCode"`
	LineAmount  float64 `json:"LineAmount"`
	Description string  `json:"Description,omitempty"`
	TaxType     string  `json:"TaxType,omitempty"`
}

// ManualJournalCreateInput is the payload for creating a manual journal.
type ManualJournalCreateInput struct {
	Narration    string              `json:"Narration"`
	Date         string              `json:"Date,omitempty"`
	JournalLines []ManualJournalLine `json:"JournalLines"`
}

type manualJournalsResponse struct {
	ManualJournals []ManualJournal `json:"ManualJournals"`
}

// JournalEntry represents an entry in the Xero journal ledger (read-only audit trail).
type JournalEntry struct {
	JournalID     string             `json:"JournalID"`
	JournalDate   string             `json:"JournalDate"`
	JournalNumber int                `json:"JournalNumber"`
	JournalLines  []JournalEntryLine `json:"JournalLines"`
}

// JournalEntryLine is a line in a journal ledger entry.
type JournalEntryLine struct {
	AccountCode string  `json:"AccountCode"`
	AccountName string  `json:"AccountName"`
	NetAmount   float64 `json:"NetAmount"`
	GrossAmount float64 `json:"GrossAmount"`
	TaxAmount   float64 `json:"TaxAmount"`
	Description string  `json:"Description,omitempty"`
}

type journalEntriesResponse struct {
	Journals []JournalEntry `json:"Journals"`
}

// ListManualJournals fetches manual journals with optional date filtering.
// If page == 0, automatically paginates through all results.
func (c *Client) ListManualJournals(fromDate, toDate string, page int) ([]ManualJournal, error) {
	if page > 0 {
		return c.listManualJournalsPage(fromDate, toDate, page)
	}
	var all []ManualJournal
	for p := 1; ; p++ {
		batch, err := c.listManualJournalsPage(fromDate, toDate, p)
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

func (c *Client) listManualJournalsPage(fromDate, toDate string, page int) ([]ManualJournal, error) {
	params := url.Values{}
	if fromDate != "" {
		params.Set("DateFrom", fromDate)
	}
	if toDate != "" {
		params.Set("DateTo", toDate)
	}
	params.Set("page", fmt.Sprintf("%d", page))
	path := "/ManualJournals?" + params.Encode()
	var resp manualJournalsResponse
	if err := c.get(path, &resp); err != nil {
		return nil, err
	}
	return resp.ManualJournals, nil
}

// GetManualJournal fetches a single manual journal by ID.
func (c *Client) GetManualJournal(id string) (*ManualJournal, error) {
	var resp manualJournalsResponse
	if err := c.get("/ManualJournals/"+url.PathEscape(id), &resp); err != nil {
		return nil, err
	}
	if len(resp.ManualJournals) == 0 {
		return nil, fmt.Errorf("manual journal not found: %s", id)
	}
	return &resp.ManualJournals[0], nil
}

// CreateManualJournal creates a new manual journal.
func (c *Client) CreateManualJournal(input ManualJournalCreateInput) (*ManualJournal, error) {
	if input.Date == "" {
		input.Date = time.Now().Format("2006-01-02")
	}
	payload := map[string]any{"ManualJournals": []ManualJournalCreateInput{input}}
	var resp manualJournalsResponse
	if err := c.put("/ManualJournals", payload, &resp); err != nil {
		return nil, err
	}
	if len(resp.ManualJournals) == 0 {
		return nil, fmt.Errorf("no manual journal returned from API")
	}
	return &resp.ManualJournals[0], nil
}

// ListJournalEntries fetches journal ledger entries (read-only audit trail).
// Uses offset-based pagination. If offset == 0, fetches all entries.
func (c *Client) ListJournalEntries(fromDate, toDate string, offset int) ([]JournalEntry, error) {
	if offset > 0 {
		return c.listJournalEntriesPage(fromDate, toDate, offset)
	}
	var all []JournalEntry
	for off := 0; ; off += 100 {
		batch, err := c.listJournalEntriesPage(fromDate, toDate, off)
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

func (c *Client) listJournalEntriesPage(fromDate, toDate string, offset int) ([]JournalEntry, error) {
	params := url.Values{}
	if fromDate != "" {
		params.Set("DateFrom", fromDate)
	}
	if toDate != "" {
		params.Set("DateTo", toDate)
	}
	params.Set("offset", fmt.Sprintf("%d", offset))
	path := "/Journals?" + params.Encode()
	var resp journalEntriesResponse
	if err := c.get(path, &resp); err != nil {
		return nil, err
	}
	return resp.Journals, nil
}
