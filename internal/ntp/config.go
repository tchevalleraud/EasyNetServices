// Package ntp manages the NTP server, backed by chrony.
package ntp

import (
	"fmt"
	"net/netip"

	"github.com/tchevalleraud/EasyNetServices/internal/validation"
)

// SourceType is the kind of upstream time source.
type SourceType string

const (
	// SourceServer is a single NTP server.
	SourceServer SourceType = "server"
	// SourcePool is a DNS name resolving to several NTP servers.
	SourcePool SourceType = "pool"
)

// Source is an upstream time source.
type Source struct {
	Type    SourceType `json:"type"`
	Address string     `json:"address"`
	// IBurst speeds up the initial synchronization.
	IBurst bool `json:"iburst"`
	// Prefer gives this source priority over the others.
	Prefer bool `json:"prefer"`
	// NTS authenticates the source with Network Time Security.
	NTS bool `json:"nts"`
}

// AccessAction tells whether clients of a network may query the server.
type AccessAction string

const (
	AccessAllow AccessAction = "allow"
	AccessDeny  AccessAction = "deny"
)

// AccessRule allows or denies a client network.
type AccessRule struct {
	Action  AccessAction `json:"action"`
	Network string       `json:"network"`
}

// Config is the NTP service configuration.
type Config struct {
	Enabled bool         `json:"enabled"`
	Sources []Source     `json:"sources"`
	Access  []AccessRule `json:"access"`
	// LocalStratum, when not zero, keeps serving time from the local clock at
	// this stratum when no upstream source is reachable (isolated networks).
	LocalStratum int `json:"localStratum"`
}

const (
	maxSources     = 32
	maxAccessRules = 256
)

// DefaultConfig returns the configuration applied on first start: public NTP
// pool as upstream, clients allowed from private networks.
func DefaultConfig() Config {
	return Config{
		Enabled: true,
		Sources: []Source{
			{Type: SourcePool, Address: "pool.ntp.org", IBurst: true},
		},
		Access: []AccessRule{
			{Action: AccessAllow, Network: "10.0.0.0/8"},
			{Action: AccessAllow, Network: "172.16.0.0/12"},
			{Action: AccessAllow, Network: "192.168.0.0/16"},
			{Action: AccessAllow, Network: "fc00::/7"},
		},
	}
}

// Normalize replaces nil slices with empty ones so the JSON form is stable.
func (c *Config) Normalize() {
	if c.Sources == nil {
		c.Sources = []Source{}
	}
	if c.Access == nil {
		c.Access = []AccessRule{}
	}
	for i := range c.Access {
		if p, err := netip.ParsePrefix(c.Access[i].Network); err == nil {
			c.Access[i].Network = p.Masked().String()
		}
	}
}

// Validate checks the configuration. It returns validation.Errors.
func (c *Config) Validate() error {
	var errs validation.Errors

	if c.Enabled && len(c.Sources) == 0 && c.LocalStratum == 0 {
		errs.Add("sources", "at least one source is required, unless a local stratum is set")
	}
	if len(c.Sources) > maxSources {
		errs.Add("sources", "at most %d sources are allowed", maxSources)
	}
	seen := map[string]bool{}
	for i, s := range c.Sources {
		field := fmt.Sprintf("sources[%d]", i)
		switch s.Type {
		case SourceServer, SourcePool:
		default:
			errs.Add(field+".type", "must be %q or %q", SourceServer, SourcePool)
		}
		if !validation.IsHost(s.Address) {
			errs.Add(field+".address", "must be a valid hostname or IP address")
		} else if seen[s.Address] {
			errs.Add(field+".address", "duplicate source")
		}
		seen[s.Address] = true
	}

	if len(c.Access) > maxAccessRules {
		errs.Add("access", "at most %d rules are allowed", maxAccessRules)
	}
	for i, r := range c.Access {
		field := fmt.Sprintf("access[%d]", i)
		switch r.Action {
		case AccessAllow, AccessDeny:
		default:
			errs.Add(field+".action", "must be %q or %q", AccessAllow, AccessDeny)
		}
		if _, err := netip.ParsePrefix(r.Network); err != nil {
			errs.Add(field+".network", "must be a network in CIDR notation, e.g. 192.168.1.0/24")
		}
	}

	if c.LocalStratum < 0 || c.LocalStratum > 15 {
		errs.Add("localStratum", "must be between 1 and 15, or 0 to disable")
	}

	return errs.Err()
}
