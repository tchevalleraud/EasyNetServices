// Package dhcp manages the DHCP service, backed by the ISC Kea DHCPv4 and
// DHCPv6 servers.
package dhcp

import (
	"fmt"
	"net"
	"net/netip"
	"regexp"
	"strings"

	"github.com/tchevalleraud/EasyNetServices/internal/dns"
	"github.com/tchevalleraud/EasyNetServices/internal/validation"
)

// Family is an IP version served by its own Kea server.
type Family string

const (
	V4 Family = "v4"
	V6 Family = "v6"
)

// ParseFamily returns the family named in an API path.
func ParseFamily(s string) (Family, bool) {
	switch Family(s) {
	case V4, V6:
		return Family(s), true
	}
	return "", false
}

func (f Family) String() string {
	if f == V6 {
		return "DHCPv6"
	}
	return "DHCPv4"
}

// Options are the network settings sent to the clients. In a subnet, an empty
// option inherits the global value.
type Options struct {
	DNSServers []string `json:"dnsServers"`
	// DomainName is the DHCPv4 domain name (option 15). DHCPv6 only knows the
	// search list.
	DomainName   string   `json:"domainName"`
	DomainSearch []string `json:"domainSearch"`
	// NTPServers are sent as NTP servers in DHCPv4 and SNTP servers in DHCPv6.
	NTPServers []string `json:"ntpServers"`
}

// FamilySettings are the global settings of the DHCPv4 or DHCPv6 server.
type FamilySettings struct {
	Enabled bool `json:"enabled"`
	// ValidLifetime is the lease time in seconds.
	ValidLifetime uint32 `json:"validLifetime"`
	// PreferredLifetime is the DHCPv6 preferred lifetime in seconds.
	PreferredLifetime uint32  `json:"preferredLifetime"`
	Options           Options `json:"options"`
}

// Settings are the global DHCP settings. Subnets are managed separately.
type Settings struct {
	// Interfaces are the network interfaces the servers listen on.
	Interfaces []string       `json:"interfaces"`
	DHCPv4     FamilySettings `json:"dhcpv4"`
	DHCPv6     FamilySettings `json:"dhcpv6"`
}

// Reservation assigns a fixed address to a client.
type Reservation struct {
	// HWAddress is the MAC address of the client.
	HWAddress string `json:"hwAddress"`
	// DUID identifies a DHCPv6 client; either it or HWAddress is required.
	DUID        string `json:"duid"`
	IPAddress   string `json:"ipAddress"`
	Hostname    string `json:"hostname"`
	Description string `json:"description"`
}

// Subnet is a network served by the DHCP server.
type Subnet struct {
	// ID identifies the subnet in Kea and in its leases; it is assigned on
	// creation and never changes.
	ID          uint32 `json:"id"`
	Subnet      string `json:"subnet"`
	Description string `json:"description"`
	// Interface is the interface of the directly connected clients. It is
	// required in DHCPv6 unless the clients are behind a relay.
	Interface string `json:"interface"`
	// Pools are address ranges ("first - last") or networks in CIDR notation.
	Pools []string `json:"pools"`
	// Gateway is the default router sent to DHCPv4 clients. DHCPv6 clients
	// learn their router from router advertisements.
	Gateway string `json:"gateway"`
	// ValidLifetime overrides the global lease time when set.
	ValidLifetime uint32        `json:"validLifetime"`
	Options       Options       `json:"options"`
	Reservations  []Reservation `json:"reservations"`
}

const (
	maxListItems    = 32
	maxSubnets      = 1024
	maxPools        = 64
	maxReservations = 4096
	maxDescription  = 200
)

// DefaultSettings returns the settings used on first start: both servers are
// disabled until an interface and a subnet are configured.
func DefaultSettings() Settings {
	s := Settings{
		Interfaces: []string{},
		DHCPv4:     FamilySettings{ValidLifetime: 86400},
		DHCPv6:     FamilySettings{ValidLifetime: 86400, PreferredLifetime: 43200},
	}
	s.Normalize()
	return s
}

// Family returns the settings of a family.
func (s *Settings) Family(f Family) *FamilySettings {
	if f == V6 {
		return &s.DHCPv6
	}
	return &s.DHCPv4
}

// Normalize canonicalizes the settings so that the stored form is stable.
func (s *Settings) Normalize() {
	if s.Interfaces == nil {
		s.Interfaces = []string{}
	}
	for i, name := range s.Interfaces {
		s.Interfaces[i] = strings.TrimSpace(name)
	}
	s.DHCPv4.PreferredLifetime = 0
	s.DHCPv4.Options.normalize()
	s.DHCPv6.Options.normalize()
}

// Validate checks the settings. It returns validation.Errors.
func (s *Settings) Validate() error {
	var errs validation.Errors
	if len(s.Interfaces) > maxListItems {
		errs.Add("interfaces", "at most %d interfaces are allowed", maxListItems)
	}
	seen := map[string]bool{}
	for i, name := range s.Interfaces {
		field := fmt.Sprintf("interfaces[%d]", i)
		switch {
		case !isInterfaceName(name):
			errs.Add(field, "must be an interface name")
		case seen[name]:
			errs.Add(field, "duplicate interface")
		}
		seen[name] = true
	}
	for _, f := range []Family{V4, V6} {
		fs := s.Family(f)
		prefix := "dhcp" + string(f)
		if fs.Enabled && len(s.Interfaces) == 0 {
			errs.Add("interfaces", "at least one interface is required to enable %s", f)
		}
		if fs.ValidLifetime < minLifetime || fs.ValidLifetime > maxLifetime {
			errs.Add(prefix+".validLifetime", "must be between %d and %d seconds", minLifetime, maxLifetime)
		}
		if f == V6 && (fs.PreferredLifetime < minLifetime || fs.PreferredLifetime > fs.ValidLifetime) {
			errs.Add(prefix+".preferredLifetime", "must be between %d seconds and the valid lifetime", minLifetime)
		}
		fs.Options.validate(&errs, prefix+".options", f)
	}
	return errs.Err()
}

const (
	minLifetime = 60
	maxLifetime = 31536000
)

var interfaceRe = regexp.MustCompile(`^[A-Za-z0-9_.:@-]{1,15}$`)

func isInterfaceName(s string) bool {
	return interfaceRe.MatchString(s)
}

func (o *Options) normalize() {
	o.DNSServers = normalizeList(o.DNSServers)
	o.DomainSearch = normalizeList(o.DomainSearch)
	o.NTPServers = normalizeList(o.NTPServers)
	o.DomainName = dns.CanonicalName(o.DomainName)
	for i, d := range o.DomainSearch {
		o.DomainSearch[i] = dns.CanonicalName(d)
	}
}

func normalizeList(list []string) []string {
	if list == nil {
		return []string{}
	}
	for i, v := range list {
		list[i] = strings.TrimSpace(v)
	}
	return list
}

func (o *Options) validate(errs *validation.Errors, prefix string, f Family) {
	validateAddresses(errs, prefix+".dnsServers", o.DNSServers, f)
	validateAddresses(errs, prefix+".ntpServers", o.NTPServers, f)
	if o.DomainName != "" {
		if f == V6 {
			errs.Add(prefix+".domainName", "is not supported by DHCPv6, use the domain search list")
		} else if !dns.IsDomainName(o.DomainName) || o.DomainName == "." {
			errs.Add(prefix+".domainName", "must be a valid domain name")
		}
	}
	if len(o.DomainSearch) > maxListItems {
		errs.Add(prefix+".domainSearch", "at most %d domains are allowed", maxListItems)
	}
	for i, d := range o.DomainSearch {
		if !dns.IsDomainName(d) || d == "." {
			errs.Add(fmt.Sprintf("%s.domainSearch[%d]", prefix, i), "must be a valid domain name")
		}
	}
}

func validateAddresses(errs *validation.Errors, field string, list []string, f Family) {
	if len(list) > maxListItems {
		errs.Add(field, "at most %d addresses are allowed", maxListItems)
	}
	for i, a := range list {
		if _, ok := parseAddr(a, f); !ok {
			errs.Add(fmt.Sprintf("%s[%d]", field, i), "must be an %s address", ipVersion(f))
		}
	}
}

func ipVersion(f Family) string {
	if f == V6 {
		return "IPv6"
	}
	return "IPv4"
}

// parseAddr parses an address of the family.
func parseAddr(s string, f Family) (netip.Addr, bool) {
	a, err := netip.ParseAddr(s)
	if err != nil || a.Zone() != "" || a.Is4() != (f == V4) {
		return netip.Addr{}, false
	}
	return a, true
}

// addrRange is an inclusive range of addresses.
type addrRange struct{ first, last netip.Addr }

func (r addrRange) contains(a netip.Addr) bool {
	return r.first.Compare(a) <= 0 && a.Compare(r.last) <= 0
}

func (r addrRange) overlaps(o addrRange) bool {
	return r.first.Compare(o.last) <= 0 && o.first.Compare(r.last) <= 0
}

func (r addrRange) String() string {
	return r.first.String() + " - " + r.last.String()
}

// prefixRange returns the addresses of a network.
func prefixRange(p netip.Prefix) addrRange {
	first := p.Masked().Addr()
	b := first.AsSlice()
	for i := p.Bits(); i < len(b)*8; i++ {
		b[i/8] |= 1 << (7 - i%8)
	}
	last, _ := netip.AddrFromSlice(b)
	return addrRange{first, last}
}

// parsePool parses "first - last" or a network in CIDR notation.
func parsePool(s string, f Family) (addrRange, bool) {
	if strings.Contains(s, "/") {
		p, err := netip.ParsePrefix(s)
		if err != nil || p.Addr().Is4() != (f == V4) {
			return addrRange{}, false
		}
		return prefixRange(p), true
	}
	first, last, ok := strings.Cut(s, "-")
	if !ok {
		return addrRange{}, false
	}
	a, ok1 := parseAddr(strings.TrimSpace(first), f)
	b, ok2 := parseAddr(strings.TrimSpace(last), f)
	if !ok1 || !ok2 || a.Compare(b) > 0 {
		return addrRange{}, false
	}
	return addrRange{a, b}, true
}

// Normalize canonicalizes a subnet so that the stored form is stable.
func (s *Subnet) Normalize(f Family) {
	s.Subnet = strings.TrimSpace(s.Subnet)
	if p, err := netip.ParsePrefix(s.Subnet); err == nil {
		s.Subnet = p.Masked().String()
	}
	s.Description = strings.TrimSpace(s.Description)
	s.Interface = strings.TrimSpace(s.Interface)
	s.Gateway = strings.TrimSpace(s.Gateway)
	s.Pools = normalizeList(s.Pools)
	for i, p := range s.Pools {
		if r, ok := parsePool(p, f); ok && !strings.Contains(p, "/") {
			s.Pools[i] = r.String()
		}
	}
	s.Options.normalize()
	if s.Reservations == nil {
		s.Reservations = []Reservation{}
	}
	for i := range s.Reservations {
		s.Reservations[i].normalize()
	}
}

func (r *Reservation) normalize() {
	r.HWAddress = strings.TrimSpace(r.HWAddress)
	if mac, err := net.ParseMAC(r.HWAddress); err == nil {
		r.HWAddress = mac.String()
	}
	r.DUID = strings.ToLower(strings.TrimSpace(r.DUID))
	r.IPAddress = strings.TrimSpace(r.IPAddress)
	r.Hostname = dns.CanonicalName(r.Hostname)
	r.Description = strings.TrimSpace(r.Description)
}

var duidRe = regexp.MustCompile(`^[0-9a-f]{2}(:[0-9a-f]{2}){2,129}$`)

// Validate checks a subnet of the family. interfaces are the interfaces the
// servers listen on. It returns validation.Errors.
func (s *Subnet) Validate(f Family, interfaces []string) error {
	var errs validation.Errors

	prefix, err := netip.ParsePrefix(s.Subnet)
	validPrefix := err == nil && prefix.Addr().Is4() == (f == V4)
	if !validPrefix {
		errs.Add("subnet", "must be an %s network in CIDR notation", ipVersion(f))
	}
	if len(s.Description) > maxDescription {
		errs.Add("description", "must be at most %d characters", maxDescription)
	}
	if s.Interface != "" && !contains(interfaces, s.Interface) {
		errs.Add("interface", "must be one of the interfaces of the DHCP settings")
	}

	if len(s.Pools) > maxPools {
		errs.Add("pools", "at most %d pools are allowed", maxPools)
	}
	var pools []addrRange
	for i, p := range s.Pools {
		field := fmt.Sprintf("pools[%d]", i)
		r, ok := parsePool(p, f)
		switch {
		case !ok:
			errs.Add(field, "must be a range \"first - last\" or a network in CIDR notation")
		case validPrefix && (!prefix.Contains(r.first) || !prefix.Contains(r.last)):
			errs.Add(field, "must be inside %s", prefix)
		default:
			for _, other := range pools {
				if r.overlaps(other) {
					errs.Add(field, "overlaps another pool")
					break
				}
			}
			pools = append(pools, r)
		}
	}

	if s.Gateway != "" {
		if f == V6 {
			errs.Add("gateway", "is not supported by DHCPv6, clients learn their router from router advertisements")
		} else if a, ok := parseAddr(s.Gateway, f); !ok {
			errs.Add("gateway", "must be an IPv4 address")
		} else if validPrefix && !prefix.Contains(a) {
			errs.Add("gateway", "must be inside %s", prefix)
		}
	}
	if s.ValidLifetime != 0 && (s.ValidLifetime < minLifetime || s.ValidLifetime > maxLifetime) {
		errs.Add("validLifetime", "must be 0 (inherit) or between %d and %d seconds", minLifetime, maxLifetime)
	}
	s.Options.validate(&errs, "options", f)

	if len(s.Reservations) > maxReservations {
		errs.Add("reservations", "at most %d reservations are allowed", maxReservations)
	}
	addresses := map[netip.Addr]bool{}
	identifiers := map[string]bool{}
	for i, r := range s.Reservations {
		field := fmt.Sprintf("reservations[%d]", i)
		if a, ok := parseAddr(r.IPAddress, f); !ok {
			errs.Add(field+".ipAddress", "must be an %s address", ipVersion(f))
		} else if validPrefix && !prefix.Contains(a) {
			errs.Add(field+".ipAddress", "must be inside %s", prefix)
		} else if addresses[a] {
			errs.Add(field+".ipAddress", "is already reserved")
		} else {
			addresses[a] = true
		}

		switch {
		case r.HWAddress == "" && r.DUID == "":
			if f == V6 {
				errs.Add(field+".duid", "a DUID or a MAC address is required")
			} else {
				errs.Add(field+".hwAddress", "is required")
			}
		case r.HWAddress != "" && r.DUID != "":
			errs.Add(field+".duid", "set either a DUID or a MAC address, not both")
		case r.HWAddress != "":
			if _, err := net.ParseMAC(r.HWAddress); err != nil || len(r.HWAddress) != 17 {
				errs.Add(field+".hwAddress", "must be a MAC address such as 00:11:22:33:44:55")
			} else if identifiers["hw:"+r.HWAddress] {
				errs.Add(field+".hwAddress", "already has a reservation in this subnet")
			}
			identifiers["hw:"+r.HWAddress] = true
		default:
			if f == V4 {
				errs.Add(field+".duid", "is only supported by DHCPv6")
			} else if !duidRe.MatchString(r.DUID) {
				errs.Add(field+".duid", "must be hexadecimal bytes separated by colons")
			} else if identifiers["duid:"+r.DUID] {
				errs.Add(field+".duid", "already has a reservation in this subnet")
			}
			identifiers["duid:"+r.DUID] = true
		}

		if r.Hostname != "" && (!dns.IsDomainName(r.Hostname) || r.Hostname == ".") {
			errs.Add(field+".hostname", "must be a valid host name")
		}
		if len(r.Description) > maxDescription {
			errs.Add(field+".description", "must be at most %d characters", maxDescription)
		}
	}
	return errs.Err()
}

func contains(list []string, v string) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}
