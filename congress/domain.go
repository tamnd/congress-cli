package congress

import (
	"context"
	"fmt"
	"strings"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/any-cli/kit/errs"
)

// domain.go exposes congress as a kit Domain: a driver that a multi-domain
// host (ant) enables with a single blank import,
//
//	import _ "github.com/tamnd/congress-cli/congress"
//
// The init below registers it; the host then routes congress:// URIs to the
// operations Register installs. The same Domain also builds the standalone
// congress binary (see cli.NewApp), so binary and host share one source of truth.
func init() { kit.Register(Domain{}) }

// Host is the canonical hostname for Congress.gov URI construction.
const Host = "api.congress.gov"

// Domain is the congress driver.
type Domain struct{}

// Info describes the scheme, the hostnames, and the binary identity.
func (Domain) Info() kit.DomainInfo {
	return kit.DomainInfo{
		Scheme: "congress",
		Hosts:  []string{Host},
		Identity: kit.Identity{
			Binary: "congress",
			Short:  "A command line for the Congress.gov API.",
			Long: `A command line for the Congress.gov API.

congress reads public US legislative data — bills, members, committees, and
amendments — over HTTPS from api.congress.gov. It shapes results into clean
records that pipe into the rest of your tools.

Get a free API key at https://api.congress.gov/sign-up/ for higher rate limits.`,
			Site: "https://api.congress.gov",
			Repo: "https://github.com/tamnd/congress-cli",
		},
	}
}

// Register installs the client factory and every operation onto app.
func (Domain) Register(app *kit.App) {
	app.SetClient(newClient)

	// bills — list legislative bills
	kit.Handle(app, kit.OpMeta{Name: "bills", Group: "read", List: true,
		Summary: "List recent legislative bills"}, listBills)

	// bill — fetch a single bill
	kit.Handle(app, kit.OpMeta{Name: "bill", Group: "read", Single: true,
		Summary: "Get a single bill by congress/type/number",
		Args: []kit.Arg{
			{Name: "congress", Help: "congress number (e.g. 118)"},
			{Name: "type", Help: "bill type: hr, s, hjres, sjres"},
			{Name: "number", Help: "bill number"},
		}}, getBill)

	// members — list members of Congress
	kit.Handle(app, kit.OpMeta{Name: "members", Group: "read", List: true,
		Summary: "List members of Congress"}, listMembers)

	// member — fetch a single member
	kit.Handle(app, kit.OpMeta{Name: "member", Group: "read", Single: true,
		Summary: "Get a single member by bioguide ID",
		Args: []kit.Arg{
			{Name: "bioguide-id", Help: "member bioguide ID (e.g. P000197)"},
		}}, getMember)

	// committees — list congressional committees
	kit.Handle(app, kit.OpMeta{Name: "committees", Group: "read", List: true,
		Summary: "List congressional committees"}, listCommittees)
}

// newClient builds the client from the host-resolved config.
func newClient(_ context.Context, cfg kit.Config) (any, error) {
	c := DefaultConfig()
	if cfg.UserAgent != "" {
		c.UserAgent = cfg.UserAgent
	}
	if cfg.Rate > 0 {
		c.Rate = cfg.Rate
	}
	if cfg.Retries > 0 {
		c.Retries = cfg.Retries
	}
	if cfg.Timeout > 0 {
		c.Timeout = cfg.Timeout
	}
	return NewClient(c), nil
}

// ---- input structs ----

type billsInput struct {
	Limit    int    `kit:"flag,inherit" help:"max results"`
	Offset   int    `kit:"flag" help:"result offset"`
	Congress int    `kit:"flag" help:"congress number (e.g. 118)"`
	Type     string `kit:"flag" help:"bill type: hr, s, hjres, sjres"`
	Key      string `kit:"flag" help:"Congress.gov API key" default:"DEMO_KEY"`
	Client   *Client `kit:"inject"`
}

type billInput struct {
	Congress string  `kit:"arg" help:"congress number (e.g. 118)"`
	Type     string  `kit:"arg" help:"bill type: hr, s, hjres, sjres"`
	Number   string  `kit:"arg" help:"bill number"`
	Key      string  `kit:"flag" help:"Congress.gov API key" default:"DEMO_KEY"`
	Client   *Client `kit:"inject"`
}

type membersInput struct {
	Limit  int    `kit:"flag,inherit" help:"max results"`
	Offset int    `kit:"flag" help:"result offset"`
	State  string `kit:"flag" help:"two-letter state code (e.g. CA)"`
	Party  string `kit:"flag" help:"party code: D, R, ID"`
	Key    string `kit:"flag" help:"Congress.gov API key" default:"DEMO_KEY"`
	Client *Client `kit:"inject"`
}

type memberInput struct {
	BioguideID string  `kit:"arg" help:"member bioguide ID (e.g. P000197)"`
	Key        string  `kit:"flag" help:"Congress.gov API key" default:"DEMO_KEY"`
	Client     *Client `kit:"inject"`
}

type committeesInput struct {
	Limit   int    `kit:"flag,inherit" help:"max results"`
	Chamber string `kit:"flag" help:"chamber: senate, house, joint"`
	Key     string `kit:"flag" help:"Congress.gov API key" default:"DEMO_KEY"`
	Client  *Client `kit:"inject"`
}

// ---- handlers ----

func listBills(ctx context.Context, in billsInput, emit func(*Bill) error) error {
	if in.Key != "" && in.Key != "DEMO_KEY" {
		in.Client.cfg.APIKey = in.Key
	}
	bills, _, err := in.Client.Bills(ctx, BillsOptions{
		Limit:    in.Limit,
		Offset:   in.Offset,
		Congress: in.Congress,
		Type:     in.Type,
	})
	if err != nil {
		return mapErr(err)
	}
	for _, b := range bills {
		if err := emit(b); err != nil {
			return err
		}
	}
	return nil
}

func getBill(ctx context.Context, in billInput, emit func(*BillDetail) error) error {
	if in.Key != "" && in.Key != "DEMO_KEY" {
		in.Client.cfg.APIKey = in.Key
	}
	var congress int
	if _, err := fmt.Sscanf(in.Congress, "%d", &congress); err != nil {
		return errs.Usage("congress must be a number, got %q", in.Congress)
	}
	b, err := in.Client.Bill(ctx, congress, in.Type, in.Number)
	if err != nil {
		return mapErr(err)
	}
	return emit(b)
}

func listMembers(ctx context.Context, in membersInput, emit func(*Member) error) error {
	if in.Key != "" && in.Key != "DEMO_KEY" {
		in.Client.cfg.APIKey = in.Key
	}
	members, _, err := in.Client.Members(ctx, MembersOptions{
		Limit:  in.Limit,
		Offset: in.Offset,
		State:  in.State,
		Party:  in.Party,
	})
	if err != nil {
		return mapErr(err)
	}
	for _, m := range members {
		if err := emit(m); err != nil {
			return err
		}
	}
	return nil
}

func getMember(ctx context.Context, in memberInput, emit func(*MemberDetail) error) error {
	if in.Key != "" && in.Key != "DEMO_KEY" {
		in.Client.cfg.APIKey = in.Key
	}
	m, err := in.Client.Member(ctx, in.BioguideID)
	if err != nil {
		return mapErr(err)
	}
	return emit(m)
}

func listCommittees(ctx context.Context, in committeesInput, emit func(*Committee) error) error {
	if in.Key != "" && in.Key != "DEMO_KEY" {
		in.Client.cfg.APIKey = in.Key
	}
	committees, _, err := in.Client.Committees(ctx, CommitteesOptions{
		Limit:   in.Limit,
		Chamber: in.Chamber,
	})
	if err != nil {
		return mapErr(err)
	}
	for _, committee := range committees {
		if err := emit(committee); err != nil {
			return err
		}
	}
	return nil
}

// ---- Resolver: pure string functions, no network ----

// Classify turns a Congress.gov URL or bare path into (type, id).
func (Domain) Classify(input string) (uriType, id string, err error) {
	input = strings.TrimSpace(input)
	// try to extract path from URL
	if strings.HasPrefix(input, "https://") || strings.HasPrefix(input, "http://") {
		// strip scheme+host
		rest := input
		for _, prefix := range []string{"https://", "http://"} {
			if strings.HasPrefix(rest, prefix) {
				rest = rest[len(prefix):]
			}
		}
		// strip host
		if slash := strings.IndexByte(rest, '/'); slash >= 0 {
			rest = rest[slash+1:]
		} else {
			rest = ""
		}
		input = rest
	}
	input = strings.Trim(input, "/")
	if input == "" {
		return "", "", errs.Usage("unrecognized congress reference: empty")
	}
	// Classify by path prefix
	switch {
	case strings.HasPrefix(input, "v3/bill/") || strings.HasPrefix(input, "bill/"):
		return "bill", input, nil
	case strings.HasPrefix(input, "v3/member/") || strings.HasPrefix(input, "member/"):
		return "member", input, nil
	case strings.HasPrefix(input, "v3/committee/") || strings.HasPrefix(input, "committee/"):
		return "committee", input, nil
	default:
		return "page", input, nil
	}
}

// Locate returns the live https URL for a (type, id).
func (Domain) Locate(uriType, id string) (string, error) {
	switch uriType {
	case "bill", "member", "committee", "page":
		return "https://" + Host + "/v3/" + strings.Trim(id, "/"), nil
	default:
		return "", errs.Usage("congress has no resource type %q", uriType)
	}
}

// mapErr converts library errors into kit error kinds with the right exit code.
func mapErr(err error) error {
	return err
}
