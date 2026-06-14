package congress

import (
	"testing"
)

// These tests are offline: they exercise the URI driver's pure string functions.
// The client's HTTP behaviour is covered in congress_test.go.

func TestDomainInfo(t *testing.T) {
	info := Domain{}.Info()
	if info.Scheme != "congress" {
		t.Errorf("Scheme = %q, want congress", info.Scheme)
	}
	if len(info.Hosts) == 0 || info.Hosts[0] != Host {
		t.Errorf("Hosts = %v, want [%s]", info.Hosts, Host)
	}
	if info.Identity.Binary != "congress" {
		t.Errorf("Identity.Binary = %q, want congress", info.Identity.Binary)
	}
}

func TestClassify(t *testing.T) {
	cases := []struct {
		in  string
		typ string
		id  string
	}{
		{"bill/118/hr/1", "bill", "bill/118/hr/1"},
		{"member/P000197", "member", "member/P000197"},
		{"committee/hsju00", "committee", "committee/hsju00"},
		{"/v3/bill/118/s/100", "bill", "v3/bill/118/s/100"},
		{"https://api.congress.gov/v3/member/P000197", "member", "v3/member/P000197"},
	}
	for _, tc := range cases {
		typ, id, err := Domain{}.Classify(tc.in)
		if err != nil {
			t.Errorf("Classify(%q) returned error: %v", tc.in, err)
			continue
		}
		if typ != tc.typ {
			t.Errorf("Classify(%q) type = %q, want %q", tc.in, typ, tc.typ)
		}
		if id != tc.id {
			t.Errorf("Classify(%q) id = %q, want %q", tc.in, id, tc.id)
		}
	}
}

func TestLocate(t *testing.T) {
	cases := []struct {
		uriType string
		id      string
		want    string
	}{
		{"bill", "bill/118/hr/1", "https://" + Host + "/v3/bill/118/hr/1"},
		{"member", "member/P000197", "https://" + Host + "/v3/member/P000197"},
		{"committee", "committee/hsju00", "https://" + Host + "/v3/committee/hsju00"},
		{"page", "v3/bill", "https://" + Host + "/v3/v3/bill"},
	}
	for _, tc := range cases {
		got, err := Domain{}.Locate(tc.uriType, tc.id)
		if err != nil {
			t.Errorf("Locate(%q, %q) error: %v", tc.uriType, tc.id, err)
			continue
		}
		if got != tc.want {
			t.Errorf("Locate(%q, %q) = %q, want %q", tc.uriType, tc.id, got, tc.want)
		}
	}

	_, err := Domain{}.Locate("unknown", "foo")
	if err == nil {
		t.Error("Locate with unknown type should return error")
	}
}
