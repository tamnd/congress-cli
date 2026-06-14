package congress_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tamnd/congress-cli/congress"
)

// newTestClient returns a Client configured to hit ts with no rate-limiting.
func newTestClient(ts *httptest.Server) *congress.Client {
	cfg := congress.DefaultConfig()
	cfg.BaseURL = ts.URL
	cfg.Rate = 0
	cfg.APIKey = "TEST"
	return congress.NewClient(cfg)
}

func TestBillsList(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/bill") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("api_key") != "TEST" {
			t.Errorf("api_key not set, got %q", r.URL.Query().Get("api_key"))
		}
		if r.URL.Query().Get("format") != "json" {
			t.Errorf("format not set, got %q", r.URL.Query().Get("format"))
		}
		resp := map[string]any{
			"bills": []map[string]any{
				{"congress": 118, "type": "HR", "number": "1", "title": "Test Bill",
					"latestAction": map[string]any{"actionDate": "2023-01-09", "text": "Passed"},
					"url": "https://api.congress.gov/v3/bill/118/hr/1"},
			},
			"pagination": map[string]any{"count": 1},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	bills, pagination, err := c.Bills(context.Background(), congress.BillsOptions{Limit: 3})
	if err != nil {
		t.Fatalf("Bills error: %v", err)
	}
	if len(bills) != 1 {
		t.Fatalf("want 1 bill, got %d", len(bills))
	}
	if bills[0].Title != "Test Bill" {
		t.Errorf("title = %q, want Test Bill", bills[0].Title)
	}
	if bills[0].Congress != 118 {
		t.Errorf("congress = %d, want 118", bills[0].Congress)
	}
	if pagination.Count != 1 {
		t.Errorf("pagination count = %d, want 1", pagination.Count)
	}
}

func TestSingleBill(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/bill/118/hr/1" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		resp := map[string]any{
			"bill": map[string]any{
				"congress": 118, "type": "HR", "number": "1",
				"title":         "Consolidated Appropriations Act",
				"latestAction":  map[string]any{"actionDate": "2023-03-09", "text": "Became Law"},
				"introducedDate": "2023-01-09",
				"originChamber":  "House",
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	b, err := c.Bill(context.Background(), 118, "hr", "1")
	if err != nil {
		t.Fatalf("Bill error: %v", err)
	}
	if b.Title != "Consolidated Appropriations Act" {
		t.Errorf("title = %q", b.Title)
	}
	if b.Congress != 118 {
		t.Errorf("congress = %d, want 118", b.Congress)
	}
	if b.OriginChamber != "House" {
		t.Errorf("originChamber = %q, want House", b.OriginChamber)
	}
}

func TestMembersList(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/member") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		resp := map[string]any{
			"members": []map[string]any{
				{"bioguideId": "P000197", "name": "Pelosi, Nancy", "state": "California",
					"partyName": "Democratic", "district": 11,
					"depiction": map[string]any{"imageUrl": "https://bioguide.congress.gov/bioguide/photo/P/P000197.jpg"}},
			},
			"pagination": map[string]any{"count": 2693},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	members, pagination, err := c.Members(context.Background(), congress.MembersOptions{Limit: 3})
	if err != nil {
		t.Fatalf("Members error: %v", err)
	}
	if len(members) != 1 {
		t.Fatalf("want 1 member, got %d", len(members))
	}
	if members[0].BioguideID != "P000197" {
		t.Errorf("bioguideId = %q, want P000197", members[0].BioguideID)
	}
	if members[0].Name != "Pelosi, Nancy" {
		t.Errorf("name = %q, want Pelosi, Nancy", members[0].Name)
	}
	if pagination.Count != 2693 {
		t.Errorf("pagination count = %d, want 2693", pagination.Count)
	}
}

func TestSingleMember(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/member/P000197" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		resp := map[string]any{
			"member": map[string]any{
				"bioguideId":    "P000197",
				"firstName":     "Nancy",
				"lastName":      "Pelosi",
				"state":         "California",
				"partyName":     "Democratic",
				"currentMember": false,
				"birthYear":     "1940",
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	m, err := c.Member(context.Background(), "P000197")
	if err != nil {
		t.Fatalf("Member error: %v", err)
	}
	if m.BioguideID != "P000197" {
		t.Errorf("bioguideId = %q, want P000197", m.BioguideID)
	}
	if m.FirstName != "Nancy" {
		t.Errorf("firstName = %q, want Nancy", m.FirstName)
	}
	if m.BirthYear != "1940" {
		t.Errorf("birthYear = %q, want 1940", m.BirthYear)
	}
}

func TestCommitteesList(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/committee") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		resp := map[string]any{
			"committees": []map[string]any{
				{"systemCode": "hsju00", "name": "House Committee on the Judiciary",
					"chamber": "House", "committeeTypeCode": "Standing"},
				{"systemCode": "ssfr00", "name": "Senate Committee on Foreign Relations",
					"chamber": "Senate", "committeeTypeCode": "Standing"},
			},
			"pagination": map[string]any{"count": 2},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	committees, pagination, err := c.Committees(context.Background(), congress.CommitteesOptions{Limit: 5})
	if err != nil {
		t.Fatalf("Committees error: %v", err)
	}
	if len(committees) != 2 {
		t.Fatalf("want 2 committees, got %d", len(committees))
	}
	if committees[0].SystemCode != "hsju00" {
		t.Errorf("systemCode = %q, want hsju00", committees[0].SystemCode)
	}
	if committees[1].Chamber != "Senate" {
		t.Errorf("chamber = %q, want Senate", committees[1].Chamber)
	}
	if pagination.Count != 2 {
		t.Errorf("pagination count = %d, want 2", pagination.Count)
	}
}

func TestBillsWithChamberFilter(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// When congress and type are set, path should include them
		resp := map[string]any{
			"bills":      []map[string]any{},
			"pagination": map[string]any{"count": 0},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	bills, _, err := c.Bills(context.Background(), congress.BillsOptions{Congress: 118, Type: "hr", Limit: 5})
	if err != nil {
		t.Fatalf("Bills with congress/type error: %v", err)
	}
	// empty list is fine
	_ = bills
}

func TestRetriesOn503(t *testing.T) {
	var hits int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		resp := map[string]any{
			"bills":      []map[string]any{},
			"pagination": map[string]any{"count": 0},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	cfg := congress.DefaultConfig()
	cfg.BaseURL = ts.URL
	cfg.Rate = 0
	cfg.APIKey = "TEST"
	cfg.Retries = 5
	c := congress.NewClient(cfg)

	_, _, err := c.Bills(context.Background(), congress.BillsOptions{})
	if err != nil {
		t.Fatalf("Bills retry error: %v", err)
	}
	if hits != 3 {
		t.Errorf("server saw %d hits, want 3", hits)
	}
}
