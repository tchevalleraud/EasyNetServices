package dns

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/tchevalleraud/EasyNetServices/internal/store"
	"github.com/tchevalleraud/EasyNetServices/internal/validation"
)

func fields(err error) map[string]bool {
	var errs validation.Errors
	errors.As(err, &errs)
	got := map[string]bool{}
	for _, e := range errs {
		got[e.Field] = true
	}
	return got
}

func TestSettingsValidate(t *testing.T) {
	def := DefaultSettings()
	if err := def.Validate(); err != nil {
		t.Fatalf("default settings must be valid: %v", err)
	}

	s := Settings{
		ListenAddresses: []string{"10.0.0.1", "eth0"},
		AllowFrom:       []string{"192.168.1.0/24", "nope"},
		Resolution:      ResolutionForward,
		Forwarders:      []Forwarder{{Address: "dns.google"}},
		ForwardZones: []ForwardZone{
			{Name: "corp.example", Servers: []string{"10.0.0.10", "10.0.0.11:5353"}},
			{Name: "corp.example", Servers: []string{}},
			{Name: "bad zone", Servers: []string{"10.0.0.10:99999"}},
		},
		DNSSEC: "sometimes",
	}
	s.Normalize()
	got := fields(s.Validate())
	for _, f := range []string{
		"listenAddresses[1]", "allowFrom[1]", "forwarders[0].address",
		"forwardZones[1].name", "forwardZones[1].servers", "forwardZones[2].name",
		"forwardZones[2].servers[0]", "dnssec",
	} {
		if !got[f] {
			t.Errorf("missing error on %s (got %v)", f, got)
		}
	}
	if got["listenAddresses[0]"] || got["forwardZones[0].servers[1]"] {
		t.Errorf("valid values must be accepted (got %v)", got)
	}

	noForwarder := DefaultSettings()
	noForwarder.Resolution = ResolutionForward
	if !fields(noForwarder.Validate())["forwarders"] {
		t.Error("forward mode without forwarders must be rejected")
	}
}

func TestRenderRecursor(t *testing.T) {
	s := DefaultSettings()
	s.Enabled = true
	s.ListenAddresses = []string{"192.168.1.2"}
	s.AllowFrom = []string{"192.168.1.0/24"}
	s.Resolution = ResolutionForward
	s.Forwarders = []Forwarder{{Address: "1.1.1.1", TLS: true}, {Address: "2606:4700:4700::1111"}}
	s.ForwardZones = []ForwardZone{
		{Name: "ad.corp", Servers: []string{"10.0.0.10"}, Recursive: true},
		{Name: "partner.example", Servers: []string{"10.1.0.1:5300"}},
		{Name: "home.lan", Servers: []string{"10.9.9.9"}},
	}
	out := renderRecursor(recursorInput{Settings: s, LocalZones: []string{"home.lan", "1.168.192.in-addr.arpa"}, SocketDir: "/run/x"})

	for _, want := range []string{
		"  forward_zones:\n  - zone: 1.168.192.in-addr.arpa\n    forwarders: ['127.0.0.1:15353']\n  - zone: home.lan\n    forwarders: ['127.0.0.1:15353']\n  - zone: partner.example\n    forwarders: ['10.1.0.1:5300']\n",
		"  forward_zones_recurse:\n  - zone: ad.corp\n    forwarders: ['10.0.0.10:53']\n  - zone: .\n    forwarders: ['1.1.1.1:853', '[2606:4700:4700::1111]:53']\n",
		"  listen: ['192.168.1.2']\n",
		"  allow_from: ['192.168.1.0/24']\n",
		"  validation: process\n",
		"  - name: home.lan\n",
		"  - name: ad.corp\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	// The user forward zone shadowed by a local zone must be ignored.
	if strings.Contains(out, "10.9.9.9") {
		t.Errorf("forward zone shadowed by a local zone must be ignored:\n%s", out)
	}

	s.ListenAddresses = nil
	s.Resolution = ResolutionRecursive
	out = renderRecursor(recursorInput{Settings: s, IPv6: true, SocketDir: "/run/x"})
	if !strings.Contains(out, "listen: ['0.0.0.0', '::']") || strings.Contains(out, "zone: .\n") {
		t.Errorf("unexpected recursive configuration:\n%s", out)
	}
}

func TestRRSetValidate(t *testing.T) {
	rr := RRSet{Name: "WWW.Home.Lan.", Type: "cname", TTL: 300, Records: []Record{{Content: "nas"}}}
	if err := rr.Validate("home.lan"); err != nil {
		t.Fatal(err)
	}
	if rr.Name != "www.home.lan" || rr.Type != "CNAME" || rr.Records[0].Content != "nas.home.lan." {
		t.Fatalf("unexpected normalization: %+v", rr)
	}

	cases := map[string]string{
		"TXT": `v=spf1 -all`, "MX": "10 mail.home.lan", "SRV": "0 5 5060 sip", "A": "192.168.1.10",
		"NS": "@", "PTR": "printer.other.lan",
	}
	want := map[string]string{
		"TXT": `"v=spf1 -all"`, "MX": "10 mail.home.lan.", "SRV": "0 5 5060 sip.home.lan.", "A": "192.168.1.10",
		"NS": "home.lan.", "PTR": "printer.other.lan.",
	}
	for typ, content := range cases {
		r := RRSet{Name: "home.lan", Type: typ, TTL: 60, Records: []Record{{Content: content}}}
		if err := r.Validate("home.lan"); err != nil {
			t.Fatalf("%s: %v", typ, err)
		}
		if r.Records[0].Content != want[typ] {
			t.Errorf("%s: got %q, want %q", typ, r.Records[0].Content, want[typ])
		}
	}

	bad := RRSet{Name: "www.other.lan", Type: "SOA", TTL: 0, Records: []Record{{Content: " "}}}
	got := fields(bad.Validate("home.lan"))
	for _, f := range []string{"name", "type", "ttl", "records[0].content"} {
		if !got[f] {
			t.Errorf("missing error on %s (got %v)", f, got)
		}
	}
	twoCNAME := RRSet{Name: "a.home.lan", Type: "CNAME", TTL: 60, Records: []Record{{Content: "b"}, {Content: "c"}}}
	if !fields(twoCNAME.Validate("home.lan"))["records"] {
		t.Error("a CNAME with two targets must be rejected")
	}
}

func TestParseStats(t *testing.T) {
	stats, ok := parseStats("100\n80\n20\n42\n90\n8\n2\n3600\n")
	if !ok || stats.Questions != 100 || stats.CacheHits != 80 || stats.UptimeSeconds != 3600 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
	if _, ok := parseStats("1\n2\n"); ok {
		t.Fatal("expected a parse failure")
	}
}

// fakePDNS emulates the parts of the PowerDNS API used by the manager.
type fakePDNS struct {
	mu    sync.Mutex
	zones map[string][]pdnsRRSet
}

func (f *fakePDNS) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r.Header.Get("X-API-Key") == "" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	const prefix = "/api/v1/servers/localhost/zones"
	zone := strings.TrimPrefix(strings.TrimPrefix(r.URL.Path, prefix), "/")
	switch {
	case r.Method == http.MethodGet && zone == "":
		list := []pdnsZone{}
		for name := range f.zones {
			list = append(list, pdnsZone{Name: name, Kind: "Native", Serial: 1})
		}
		json.NewEncoder(w).Encode(list)
	case r.Method == http.MethodPost && zone == "":
		var z pdnsZone
		json.NewDecoder(r.Body).Decode(&z)
		if _, ok := f.zones[z.Name]; ok {
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(map[string]string{"error": "Conflict"})
			return
		}
		f.zones[z.Name] = []pdnsRRSet{{Name: z.Name, Type: "SOA", TTL: 3600, Records: []pdnsRecord{{Content: "ns. h. 1 1 1 1 1"}}}}
		w.WriteHeader(http.StatusCreated)
	case zone != "":
		rrsets, ok := f.zones[zone]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		switch r.Method {
		case http.MethodGet:
			json.NewEncoder(w).Encode(pdnsZone{Name: zone, Kind: "Native", Serial: 2, RRSets: rrsets})
		case http.MethodDelete:
			delete(f.zones, zone)
			w.WriteHeader(http.StatusNoContent)
		case http.MethodPatch:
			var body struct{ RRSets []pdnsRRSet }
			json.NewDecoder(r.Body).Decode(&body)
			for _, rr := range body.RRSets {
				if rr.Type == "A" && len(rr.Records) > 0 && strings.HasPrefix(rr.Records[0].Content, "999") {
					w.WriteHeader(http.StatusUnprocessableEntity)
					json.NewEncoder(w).Encode(map[string]string{"error": "unable to parse IP address"})
					return
				}
				kept := rrsets[:0]
				for _, existing := range rrsets {
					if existing.Name != rr.Name || existing.Type != rr.Type {
						kept = append(kept, existing)
					}
				}
				rrsets = kept
				if rr.ChangeType == "REPLACE" {
					rrsets = append(rrsets, rr)
				}
			}
			f.zones[zone] = rrsets
			w.WriteHeader(http.StatusNoContent)
		}
	}
}

type fakeSupervisor struct{ up map[string]bool }

func (f *fakeSupervisor) Available() bool { return true }
func (f *fakeSupervisor) Up(_ context.Context, n string) error {
	f.up[n] = true
	return nil
}
func (f *fakeSupervisor) Down(_ context.Context, n string) error {
	f.up[n] = false
	return nil
}
func (f *fakeSupervisor) Restart(_ context.Context, n string) error {
	f.up[n] = true
	return nil
}
func (f *fakeSupervisor) IsUp(_ context.Context, n string) (bool, error) { return f.up[n], nil }

func TestManager(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	st, err := store.Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	pdns := httptest.NewServer(&fakePDNS{zones: map[string][]pdnsRRSet{}})
	defer pdns.Close()
	sup := &fakeSupervisor{up: map[string]bool{}}
	var commands []string
	m := NewManager(st, sup, dir, slog.New(slog.NewTextHandler(io.Discard, nil)))
	m.SetRunner(func(_ context.Context, name string, args ...string) ([]byte, error) {
		commands = append(commands, name+" "+strings.Join(args[1:], " "))
		return nil, nil
	})
	m.SetProber(func(addr string) error {
		if addr == "10.0.0.53" {
			return errors.New("address already in use")
		}
		return nil
	})

	if err := m.Init(ctx); err != nil {
		t.Fatal(err)
	}
	m.SetAuthURL(pdns.URL)
	if err := m.StartResolver(ctx); err != nil {
		t.Fatal(err)
	}
	if !sup.up[authService] || sup.up[recursorService] {
		t.Fatalf("expected auth up and recursor down by default, got %v", sup.up)
	}

	s := DefaultSettings()
	s.Enabled = true
	s.ListenAddresses = []string{"10.0.0.53"}
	if !fields(func() error { _, err := m.UpdateSettings(ctx, s); return err }())["listenAddresses[0]"] {
		t.Fatal("a busy listen address must be reported")
	}
	s.ListenAddresses = []string{"10.0.0.1"}
	if _, err := m.UpdateSettings(ctx, s); err != nil {
		t.Fatal(err)
	}
	if !sup.up[recursorService] {
		t.Fatal("recursor must be started once enabled")
	}

	if _, err := m.CreateZone(ctx, NewZone{Name: "Home.LAN."}); err != nil {
		t.Fatal(err)
	}
	if _, err := m.CreateZone(ctx, NewZone{Name: "home.lan"}); !errors.Is(err, ErrZoneExists) {
		t.Fatalf("expected ErrZoneExists, got %v", err)
	}
	if all := strings.Join(commands, "\n"); !strings.Contains(all, "rec_control reload-zones") || !strings.Contains(all, "rec_control add-nta home.lan") {
		t.Fatalf("creating a zone must reload the resolver zones and add a trust anchor, got %v", commands)
	}

	if _, err := m.ReplaceRRSet(ctx, "home.lan", RRSet{Name: "nas.home.lan", Type: "A", TTL: 300, Records: []Record{{Content: "192.168.1.10"}}}); err != nil {
		t.Fatal(err)
	}
	var rejected *RejectedError
	if _, err := m.ReplaceRRSet(ctx, "home.lan", RRSet{Name: "bad.home.lan", Type: "A", TTL: 300, Records: []Record{{Content: "999.1.1.1"}}}); !errors.As(err, &rejected) {
		t.Fatalf("expected a rejected record, got %v", err)
	}
	zone, err := m.Zone(ctx, "home.lan")
	if err != nil {
		t.Fatal(err)
	}
	if len(zone.RRSets) != 2 || zone.RRSets[1].Name != "nas.home.lan" {
		t.Fatalf("unexpected zone: %+v", zone)
	}
	if !strings.Contains(strings.Join(commands, "\n"), "rec_control wipe-cache home.lan.$") {
		t.Fatalf("changing records must wipe the cache, got %v", commands)
	}

	if err := m.DeleteRRSet(ctx, "home.lan", "nas.home.lan", "a"); err != nil {
		t.Fatal(err)
	}
	if err := m.DeleteZone(ctx, "home.lan"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(commands, "\n"), "rec_control clear-nta home.lan") {
		t.Fatalf("deleting a zone must clear its trust anchor, got %v", commands)
	}
	if _, err := m.Zone(ctx, "home.lan"); !errors.Is(err, ErrZoneNotFound) {
		t.Fatalf("expected ErrZoneNotFound, got %v", err)
	}
}
