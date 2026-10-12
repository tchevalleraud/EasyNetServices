package dns

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var (
	// ErrZoneNotFound is returned when a zone does not exist.
	ErrZoneNotFound = errors.New("zone not found")
	// ErrZoneExists is returned when creating a zone that already exists.
	ErrZoneExists = errors.New("zone already exists")
	// ErrAuthUnavailable is returned when the authoritative server cannot be reached.
	ErrAuthUnavailable = errors.New("authoritative server unavailable")
)

// RejectedError is returned when PowerDNS refuses a change, typically an
// invalid record content.
type RejectedError struct {
	Message string
}

func (e *RejectedError) Error() string { return e.Message }

// pdnsClient talks to the HTTP API of the PowerDNS Authoritative Server.
type pdnsClient struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

func newPDNSClient(baseURL, apiKey string) *pdnsClient {
	return &pdnsClient{baseURL: baseURL, apiKey: apiKey, http: &http.Client{Timeout: 10 * time.Second}}
}

type pdnsZone struct {
	ID          string      `json:"id,omitempty"`
	Name        string      `json:"name"`
	Kind        string      `json:"kind"`
	Serial      uint32      `json:"serial,omitempty"`
	Nameservers []string    `json:"nameservers,omitempty"`
	RRSets      []pdnsRRSet `json:"rrsets,omitempty"`
}

type pdnsRRSet struct {
	Name       string       `json:"name"`
	Type       string       `json:"type"`
	TTL        uint32       `json:"ttl,omitempty"`
	ChangeType string       `json:"changetype,omitempty"`
	Records    []pdnsRecord `json:"records"`
}

type pdnsRecord struct {
	Content  string `json:"content"`
	Disabled bool   `json:"disabled"`
}

func (c *pdnsClient) zoneURL(zone string) string {
	return "/zones/" + url.PathEscape(fqdn(zone))
}

func (c *pdnsClient) listZones(ctx context.Context) ([]pdnsZone, error) {
	var zones []pdnsZone
	err := c.do(ctx, http.MethodGet, "/zones", nil, &zones)
	return zones, err
}

func (c *pdnsClient) getZone(ctx context.Context, zone string) (*pdnsZone, error) {
	var z pdnsZone
	if err := c.do(ctx, http.MethodGet, c.zoneURL(zone), nil, &z); err != nil {
		return nil, err
	}
	return &z, nil
}

func (c *pdnsClient) createZone(ctx context.Context, z pdnsZone) error {
	if z.Nameservers == nil {
		z.Nameservers = []string{}
	}
	return c.do(ctx, http.MethodPost, "/zones", z, nil)
}

func (c *pdnsClient) deleteZone(ctx context.Context, zone string) error {
	return c.do(ctx, http.MethodDelete, c.zoneURL(zone), nil, nil)
}

func (c *pdnsClient) patchRRSets(ctx context.Context, zone string, rrsets []pdnsRRSet) error {
	return c.do(ctx, http.MethodPatch, c.zoneURL(zone), map[string]any{"rrsets": rrsets}, nil)
}

func (c *pdnsClient) do(ctx context.Context, method, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+"/api/v1/servers/localhost"+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("X-API-Key", c.apiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrAuthUnavailable, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return err
	}

	if resp.StatusCode >= 300 {
		var e struct {
			Error string `json:"error"`
		}
		json.Unmarshal(data, &e)
		switch {
		case resp.StatusCode == http.StatusNotFound:
			return ErrZoneNotFound
		case resp.StatusCode == http.StatusConflict:
			return ErrZoneExists
		case resp.StatusCode == http.StatusUnprocessableEntity || resp.StatusCode == http.StatusBadRequest:
			// Drop the hint aimed at PowerDNS administrators.
			return &RejectedError{Message: strings.Replace(e.Error, " (try 'pdnsutil zone check')", "", 1)}
		default:
			return fmt.Errorf("powerdns: HTTP %d: %s", resp.StatusCode, e.Error)
		}
	}
	if out != nil && len(data) > 0 {
		return json.Unmarshal(data, out)
	}
	return nil
}
