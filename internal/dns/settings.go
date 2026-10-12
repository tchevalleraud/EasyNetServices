// Package dns manages the DNS service, backed by PowerDNS: the Authoritative
// Server hosts the local zones and the Recursor answers the clients.
package dns

import (
	"fmt"
	"net/netip"
	"strings"

	"github.com/tchevalleraud/EasyNetServices/internal/validation"
)

// ResolutionMode tells how names outside the local and forward zones are resolved.
type ResolutionMode string

const (
	// ResolutionRecursive resolves names from the root servers.
	ResolutionRecursive ResolutionMode = "recursive"
	// ResolutionForward sends queries to upstream resolvers.
	ResolutionForward ResolutionMode = "forward"
)

// DNSSECMode is the DNSSEC validation mode of the resolver.
type DNSSECMode string

const (
	DNSSECOff DNSSECMode = "off"
	// DNSSECProcess validates when the client asks for it (DO or AD bit).
	DNSSECProcess DNSSECMode = "process"
	// DNSSECValidate validates every answer and refuses bogus ones.
	DNSSECValidate DNSSECMode = "validate"
)

// Forwarder is an upstream resolver.
type Forwarder struct {
	Address string `json:"address"`
	// TLS uses DNS over TLS on port 853.
	TLS bool `json:"tls"`
}

// ForwardZone sends the queries for a domain to specific servers, e.g. the
// domain controllers of an Active Directory domain.
type ForwardZone struct {
	Name    string   `json:"name"`
	Servers []string `json:"servers"`
	// Recursive is set when the servers are resolvers (queries are sent with
	// recursion desired); otherwise they are authoritative for the domain.
	Recursive bool `json:"recursive"`
}

// Settings is the resolver configuration. Local zones are managed separately.
type Settings struct {
	Enabled bool `json:"enabled"`
	// ListenAddresses are the IP addresses to listen on; empty means all.
	ListenAddresses []string       `json:"listenAddresses"`
	AllowFrom       []string       `json:"allowFrom"`
	Resolution      ResolutionMode `json:"resolution"`
	Forwarders      []Forwarder    `json:"forwarders"`
	ForwardZones    []ForwardZone  `json:"forwardZones"`
	DNSSEC          DNSSECMode     `json:"dnssec"`
}

const (
	maxListItems  = 64
	maxACLEntries = 256
)

// DefaultSettings returns the settings used on first start. The resolver is
// disabled until the operator enables it, since port 53 is often already used
// on the host.
func DefaultSettings() Settings {
	return Settings{
		Enabled:         false,
		ListenAddresses: []string{},
		AllowFrom: []string{
			"127.0.0.0/8", "::1/128",
			"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16",
			"fc00::/7", "fe80::/10",
		},
		Resolution:   ResolutionRecursive,
		Forwarders:   []Forwarder{},
		ForwardZones: []ForwardZone{},
		DNSSEC:       DNSSECProcess,
	}
}

// Normalize canonicalizes the settings so that the stored form is stable.
func (s *Settings) Normalize() {
	if s.ListenAddresses == nil {
		s.ListenAddresses = []string{}
	}
	if s.AllowFrom == nil {
		s.AllowFrom = []string{}
	}
	if s.Forwarders == nil {
		s.Forwarders = []Forwarder{}
	}
	if s.ForwardZones == nil {
		s.ForwardZones = []ForwardZone{}
	}
	for i, n := range s.AllowFrom {
		if p, err := parsePrefix(n); err == nil {
			s.AllowFrom[i] = p.String()
		}
	}
	for i := range s.ForwardZones {
		s.ForwardZones[i].Name = CanonicalName(s.ForwardZones[i].Name)
		if s.ForwardZones[i].Servers == nil {
			s.ForwardZones[i].Servers = []string{}
		}
	}
}

// Validate checks the settings. It returns validation.Errors.
func (s *Settings) Validate() error {
	var errs validation.Errors

	if len(s.ListenAddresses) > maxListItems {
		errs.Add("listenAddresses", "at most %d addresses are allowed", maxListItems)
	}
	for i, a := range s.ListenAddresses {
		if _, err := netip.ParseAddr(a); err != nil {
			errs.Add(fmt.Sprintf("listenAddresses[%d]", i), "must be an IP address")
		}
	}

	if len(s.AllowFrom) > maxACLEntries {
		errs.Add("allowFrom", "at most %d networks are allowed", maxACLEntries)
	}
	for i, n := range s.AllowFrom {
		if _, err := parsePrefix(n); err != nil {
			errs.Add(fmt.Sprintf("allowFrom[%d]", i), "must be an IP address or a network in CIDR notation")
		}
	}

	switch s.Resolution {
	case ResolutionRecursive:
	case ResolutionForward:
		if len(s.Forwarders) == 0 {
			errs.Add("forwarders", "at least one forwarder is required in forward mode")
		}
	default:
		errs.Add("resolution", "must be %q or %q", ResolutionRecursive, ResolutionForward)
	}
	if len(s.Forwarders) > maxListItems {
		errs.Add("forwarders", "at most %d forwarders are allowed", maxListItems)
	}
	for i, f := range s.Forwarders {
		if _, err := netip.ParseAddr(f.Address); err != nil {
			errs.Add(fmt.Sprintf("forwarders[%d].address", i), "must be an IP address")
		}
	}

	if len(s.ForwardZones) > maxListItems {
		errs.Add("forwardZones", "at most %d forward zones are allowed", maxListItems)
	}
	seen := map[string]bool{}
	for i, z := range s.ForwardZones {
		field := fmt.Sprintf("forwardZones[%d]", i)
		if !IsDomainName(z.Name) || z.Name == "." {
			errs.Add(field+".name", "must be a valid domain name")
		} else if seen[z.Name] {
			errs.Add(field+".name", "duplicate forward zone")
		}
		seen[z.Name] = true
		if len(z.Servers) == 0 {
			errs.Add(field+".servers", "at least one server is required")
		}
		for j, srv := range z.Servers {
			if _, err := parseServer(srv); err != nil {
				errs.Add(fmt.Sprintf("%s.servers[%d]", field, j), "must be an IP address, optionally with a port")
			}
		}
	}

	switch s.DNSSEC {
	case DNSSECOff, DNSSECProcess, DNSSECValidate:
	default:
		errs.Add("dnssec", "must be %q, %q or %q", DNSSECOff, DNSSECProcess, DNSSECValidate)
	}

	return errs.Err()
}

// parsePrefix accepts a network in CIDR notation or a single address.
func parsePrefix(s string) (netip.Prefix, error) {
	if !strings.Contains(s, "/") {
		a, err := netip.ParseAddr(s)
		if err != nil {
			return netip.Prefix{}, err
		}
		return netip.PrefixFrom(a, a.BitLen()), nil
	}
	p, err := netip.ParsePrefix(s)
	return p.Masked(), err
}

// parseServer accepts "address" or "address:port" ("[v6]:port" for IPv6) and
// returns it with an explicit port.
func parseServer(s string) (netip.AddrPort, error) {
	if ap, err := netip.ParseAddrPort(s); err == nil {
		return ap, nil
	}
	a, err := netip.ParseAddr(s)
	if err != nil {
		return netip.AddrPort{}, err
	}
	return netip.AddrPortFrom(a, 53), nil
}
