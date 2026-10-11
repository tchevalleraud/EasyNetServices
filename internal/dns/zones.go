package dns

import (
	"fmt"
	"strings"

	"github.com/tchevalleraud/EasyNetServices/internal/validation"
)

// Zone summarizes a local zone.
type Zone struct {
	Name   string `json:"name"`
	Kind   string `json:"kind"`
	Serial uint32 `json:"serial"`
}

// ZoneDetail is a local zone with its records.
type ZoneDetail struct {
	Zone
	RRSets []RRSet `json:"rrsets"`
}

// Record is a single value of an RRSet.
type Record struct {
	Content  string `json:"content"`
	Disabled bool   `json:"disabled"`
}

// RRSet groups the records sharing a name and a type.
type RRSet struct {
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	TTL     uint32   `json:"ttl"`
	Records []Record `json:"records"`
}

// NewZone is the request to create a local zone.
type NewZone struct {
	Name string `json:"name"`
	// Nameservers are the NS records of the zone apex; optional.
	Nameservers []string `json:"nameservers"`
}

// EditableTypes are the record types that can be managed. SOA is maintained
// automatically.
var EditableTypes = []string{"A", "AAAA", "CAA", "CNAME", "MX", "NS", "PTR", "SRV", "TXT"}

const (
	minTTL     = 1
	maxTTL     = 2147483647
	maxRecords = 100
)

func isEditableType(t string) bool {
	for _, e := range EditableTypes {
		if e == t {
			return true
		}
	}
	return false
}

// Validate checks a zone creation request and canonicalizes it.
func (z *NewZone) Validate() error {
	var errs validation.Errors
	z.Name = CanonicalName(z.Name)
	if !IsDomainName(z.Name) || z.Name == "." {
		errs.Add("name", "must be a valid domain name")
	}
	for i, ns := range z.Nameservers {
		ns = CanonicalName(ns)
		z.Nameservers[i] = ns
		if !IsDomainName(ns) || ns == "." {
			errs.Add(fmt.Sprintf("nameservers[%d]", i), "must be a valid host name")
		}
	}
	return errs.Err()
}

// Validate checks an RRSet of zone and canonicalizes its name, type and
// record contents.
func (r *RRSet) Validate(zone string) error {
	var errs validation.Errors
	r.Name = CanonicalName(r.Name)
	r.Type = strings.ToUpper(strings.TrimSpace(r.Type))

	if !IsDomainName(r.Name) || !InZone(r.Name, zone) {
		errs.Add("name", "must be %s or one of its subdomains", zone)
	}
	if !isEditableType(r.Type) {
		errs.Add("type", "must be one of %s", strings.Join(EditableTypes, ", "))
	}
	if r.TTL < minTTL || r.TTL > maxTTL {
		errs.Add("ttl", "must be between %d and %d seconds", minTTL, maxTTL)
	}
	if len(r.Records) == 0 {
		errs.Add("records", "at least one record is required")
	}
	if len(r.Records) > maxRecords {
		errs.Add("records", "at most %d records are allowed", maxRecords)
	}
	if r.Type == "CNAME" && len(r.Records) > 1 {
		errs.Add("records", "a CNAME can only have one target")
	}
	for i := range r.Records {
		content := normalizeContent(r.Type, r.Records[i].Content, zone)
		r.Records[i].Content = content
		if content == "" {
			errs.Add(fmt.Sprintf("records[%d].content", i), "must not be empty")
		}
	}
	return errs.Err()
}

// normalizeContent fixes the common mistakes in record contents: missing
// quotes around TXT values and missing trailing dots on host names, which
// PowerDNS requires. A host name without any dot, or "@", is relative to the
// zone, so that "nas" means nas.<zone>.
func normalizeContent(rrtype, content, zone string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}
	fields := strings.Fields(content)
	switch rrtype {
	case "TXT":
		if !strings.HasPrefix(content, `"`) {
			return `"` + strings.ReplaceAll(content, `"`, `\"`) + `"`
		}
	case "CNAME", "NS":
		if len(fields) == 1 {
			return hostName(fields[0], zone)
		}
	case "PTR":
		// PTR records live in reverse zones: their target is always absolute.
		if len(fields) == 1 && !strings.HasSuffix(fields[0], ".") {
			return strings.ToLower(fields[0]) + "."
		}
	case "MX":
		if len(fields) == 2 {
			return fields[0] + " " + hostName(fields[1], zone)
		}
	case "SRV":
		if len(fields) == 4 {
			return strings.Join(fields[:3], " ") + " " + hostName(fields[3], zone)
		}
	}
	return content
}

// hostName returns the fully qualified form of a host name in zone.
func hostName(host, zone string) string {
	host = strings.ToLower(host)
	switch {
	case host == "@":
		return fqdn(zone)
	case strings.HasSuffix(host, "."):
		return host
	case !strings.Contains(host, "."):
		return host + "." + fqdn(zone)
	default:
		return host + "."
	}
}
