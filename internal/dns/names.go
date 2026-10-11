package dns

import (
	"regexp"
	"strings"
)

// Labels may contain underscores for service records such as _sip._tcp.
var labelRe = regexp.MustCompile(`^(?i)([a-z0-9_]|[a-z0-9_][a-z0-9_-]{0,61}[a-z0-9])$`)

// CanonicalName lowercases a domain name and removes its trailing dot. The root
// zone is returned as ".".
func CanonicalName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "." {
		return name
	}
	return strings.TrimSuffix(name, ".")
}

// IsDomainName reports whether name (in canonical form) is a valid domain name.
func IsDomainName(name string) bool {
	if name == "." {
		return true
	}
	if name == "" || len(name) > 253 {
		return false
	}
	for _, label := range strings.Split(name, ".") {
		if !labelRe.MatchString(label) {
			return false
		}
	}
	return true
}

// InZone reports whether name is the apex of zone or one of its subdomains.
func InZone(name, zone string) bool {
	return name == zone || strings.HasSuffix(name, "."+zone)
}

// fqdn returns name with a trailing dot, as expected by PowerDNS.
func fqdn(name string) string {
	if name == "." {
		return name
	}
	return name + "."
}
