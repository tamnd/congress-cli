// Package congress is the library behind the congress command line:
// the HTTP client, request shaping, and typed data models for the Congress.gov API.
//
// The Client sets a real User-Agent, paces requests to stay polite, and retries
// transient failures (429 and 5xx). All endpoint calls append api_key and format=json
// automatically via the get() helper.
package congress

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Config holds the client configuration.
type Config struct {
	BaseURL   string
	UserAgent string
	APIKey    string
	Rate      time.Duration
	Timeout   time.Duration
	Retries   int
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		BaseURL:   "https://api.congress.gov/v3",
		UserAgent: "congress-cli/0.1.0 (github.com/tamnd/congress-cli)",
		APIKey:    "DEMO_KEY",
		Rate:      200 * time.Millisecond,
		Timeout:   30 * time.Second,
		Retries:   3,
	}
}

// Client talks to the Congress.gov API over HTTPS.
type Client struct {
	http *http.Client
	cfg  Config

	mu   sync.Mutex
	last time.Time
}

// NewClient returns a Client using cfg.
func NewClient(cfg Config) *Client {
	return &Client{
		http: &http.Client{Timeout: cfg.Timeout},
		cfg:  cfg,
	}
}

// get fetches rawURL, automatically appending api_key and format=json.
func (c *Client) get(ctx context.Context, rawURL string) ([]byte, error) {
	sep := "?"
	if strings.Contains(rawURL, "?") {
		sep = "&"
	}
	rawURL += sep + "api_key=" + c.cfg.APIKey + "&format=json"

	var lastErr error
	for attempt := 0; attempt <= c.cfg.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		body, retry, err := c.do(ctx, rawURL)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("get %s: %w", rawURL, lastErr)
}

func (c *Client) do(ctx context.Context, url string) (body []byte, retry bool, err error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.cfg.UserAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("http %d", resp.StatusCode)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, true, err
	}
	return b, false, nil
}

// pace blocks until at least Rate has elapsed since the previous request.
func (c *Client) pace() {
	if c.cfg.Rate <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if wait := c.cfg.Rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

func backoff(attempt int) time.Duration {
	d := time.Duration(attempt) * 500 * time.Millisecond
	if d > 5*time.Second {
		d = 5 * time.Second
	}
	return d
}

// ---- Data models ----

// LatestAction describes the most recent legislative action on a bill.
type LatestAction struct {
	ActionDate string `json:"actionDate"`
	Text       string `json:"text"`
}

// Bill represents a legislative bill from Congress.gov.
type Bill struct {
	Congress     int          `json:"congress"`
	Type         string       `json:"type"`
	Number       string       `json:"number"`
	Title        string       `json:"title"`
	LatestAction LatestAction `json:"latestAction"`
	URL          string       `json:"url"`
}

// BillDetail holds extended bill information returned by the single-bill endpoint.
type BillDetail struct {
	Congress     int          `json:"congress"`
	Type         string       `json:"type"`
	Number       string       `json:"number"`
	Title        string       `json:"title"`
	LatestAction LatestAction `json:"latestAction"`
	URL          string       `json:"url,omitempty"`
	// Extended fields
	Sponsors          []Sponsor `json:"sponsors,omitempty"`
	PolicyArea        *struct {
		Name string `json:"name"`
	} `json:"policyArea,omitempty"`
	Introduced        string `json:"introducedDate,omitempty"`
	OriginChamber     string `json:"originChamber,omitempty"`
	OriginChamberCode string `json:"originChamberCode,omitempty"`
	UpdateDate        string `json:"updateDate,omitempty"`
}

// Sponsor is a bill sponsor reference.
type Sponsor struct {
	BioguideID string `json:"bioguideId"`
	FirstName  string `json:"firstName"`
	LastName   string `json:"lastName"`
	Party      string `json:"party"`
	State      string `json:"state"`
}

// Depiction holds a member's image information.
type Depiction struct {
	ImageURL  string `json:"imageUrl"`
	Copyright string `json:"copyright,omitempty"`
}

// Member represents a member of Congress.
type Member struct {
	BioguideID string    `json:"bioguideId"`
	Name       string    `json:"name"`
	State      string    `json:"state"`
	PartyName  string    `json:"partyName"`
	District   int       `json:"district,omitempty"`
	Depiction  Depiction `json:"depiction,omitempty"`
	URL        string    `json:"url,omitempty"`
}

// MemberDetail holds extended member information.
type MemberDetail struct {
	BioguideID    string    `json:"bioguideId"`
	FirstName     string    `json:"firstName"`
	LastName      string    `json:"lastName"`
	DirectOrderName string  `json:"directOrderName,omitempty"`
	State         string    `json:"state"`
	PartyName     string    `json:"partyName"`
	District      int       `json:"district,omitempty"`
	Depiction     Depiction `json:"depiction,omitempty"`
	BirthYear     string    `json:"birthYear,omitempty"`
	CurrentMember bool      `json:"currentMember,omitempty"`
	Leadership    []struct {
		Type string `json:"type"`
	} `json:"leadership,omitempty"`
}

// Committee represents a congressional committee.
type Committee struct {
	SystemCode        string `json:"systemCode"`
	Name              string `json:"name"`
	Chamber           string `json:"chamber"`
	CommitteeTypeCode string `json:"committeeTypeCode"`
	URL               string `json:"url,omitempty"`
}

// Pagination holds paging metadata from list responses.
type Pagination struct {
	Count  int    `json:"count"`
	Next   string `json:"next,omitempty"`
	Prev   string `json:"prev,omitempty"`
}

// ---- API methods ----

// BillsOptions controls the /bill list call.
type BillsOptions struct {
	Limit    int
	Offset   int
	Congress int    // 0 = all congresses
	Type     string // hr, s, hjres, sjres, etc.
}

// Bills lists legislative bills.
func (c *Client) Bills(ctx context.Context, opts BillsOptions) ([]*Bill, *Pagination, error) {
	u := c.cfg.BaseURL + "/bill"
	sep := "?"
	add := func(k, v string) {
		u += sep + k + "=" + v
		sep = "&"
	}
	if opts.Congress > 0 && opts.Type != "" {
		u = fmt.Sprintf("%s/bill/%d/%s", c.cfg.BaseURL, opts.Congress, opts.Type)
	} else if opts.Congress > 0 {
		u = fmt.Sprintf("%s/bill/%d", c.cfg.BaseURL, opts.Congress)
	}
	if opts.Limit > 0 {
		add("limit", fmt.Sprintf("%d", opts.Limit))
	}
	if opts.Offset > 0 {
		add("offset", fmt.Sprintf("%d", opts.Offset))
	}

	body, err := c.get(ctx, u)
	if err != nil {
		return nil, nil, err
	}

	var resp struct {
		Bills      []*Bill    `json:"bills"`
		Pagination Pagination `json:"pagination"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, nil, fmt.Errorf("decode bills: %w", err)
	}
	return resp.Bills, &resp.Pagination, nil
}

// Bill fetches a single bill by congress/type/number.
func (c *Client) Bill(ctx context.Context, congress int, billType, number string) (*BillDetail, error) {
	u := fmt.Sprintf("%s/bill/%d/%s/%s", c.cfg.BaseURL, congress, strings.ToLower(billType), number)
	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Bill BillDetail `json:"bill"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode bill: %w", err)
	}
	return &resp.Bill, nil
}

// MembersOptions controls the /member list call.
type MembersOptions struct {
	Limit  int
	Offset int
	State  string // two-letter state code
	Party  string // D, R, ID, etc.
}

// Members lists members of Congress.
func (c *Client) Members(ctx context.Context, opts MembersOptions) ([]*Member, *Pagination, error) {
	u := c.cfg.BaseURL + "/member"
	sep := "?"
	add := func(k, v string) {
		u += sep + k + "=" + v
		sep = "&"
	}
	if opts.Limit > 0 {
		add("limit", fmt.Sprintf("%d", opts.Limit))
	}
	if opts.Offset > 0 {
		add("offset", fmt.Sprintf("%d", opts.Offset))
	}
	if opts.State != "" {
		add("stateCode", opts.State)
	}
	if opts.Party != "" {
		add("partyCode", opts.Party)
	}

	body, err := c.get(ctx, u)
	if err != nil {
		return nil, nil, err
	}

	var resp struct {
		Members    []*Member  `json:"members"`
		Pagination Pagination `json:"pagination"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, nil, fmt.Errorf("decode members: %w", err)
	}
	return resp.Members, &resp.Pagination, nil
}

// Member fetches a single member by bioguide ID.
func (c *Client) Member(ctx context.Context, bioguideID string) (*MemberDetail, error) {
	u := fmt.Sprintf("%s/member/%s", c.cfg.BaseURL, bioguideID)
	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Member MemberDetail `json:"member"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode member: %w", err)
	}
	return &resp.Member, nil
}

// CommitteesOptions controls the /committee list call.
type CommitteesOptions struct {
	Limit   int
	Offset  int
	Chamber string // senate, house, joint
}

// Committees lists congressional committees.
func (c *Client) Committees(ctx context.Context, opts CommitteesOptions) ([]*Committee, *Pagination, error) {
	u := c.cfg.BaseURL + "/committee"
	sep := "?"
	add := func(k, v string) {
		u += sep + k + "=" + v
		sep = "&"
	}
	if opts.Chamber != "" {
		u = fmt.Sprintf("%s/committee/%s", c.cfg.BaseURL, strings.ToLower(opts.Chamber))
	}
	if opts.Limit > 0 {
		add("limit", fmt.Sprintf("%d", opts.Limit))
	}
	if opts.Offset > 0 {
		add("offset", fmt.Sprintf("%d", opts.Offset))
	}

	body, err := c.get(ctx, u)
	if err != nil {
		return nil, nil, err
	}

	var resp struct {
		Committees []*Committee `json:"committees"`
		Pagination Pagination   `json:"pagination"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, nil, fmt.Errorf("decode committees: %w", err)
	}
	return resp.Committees, &resp.Pagination, nil
}
