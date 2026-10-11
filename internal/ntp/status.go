package ntp

import (
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"
)

// ServiceState is the state of the NTP daemon.
type ServiceState string

const (
	StateRunning  ServiceState = "running"
	StateStopped  ServiceState = "stopped"
	StateDisabled ServiceState = "disabled"
	// StateUnavailable means the daemon cannot be managed, e.g. when the server
	// runs outside of the container.
	StateUnavailable ServiceState = "unavailable"
)

// Status is the runtime state of the NTP service.
type Status struct {
	State        ServiceState   `json:"state"`
	Synchronized bool           `json:"synchronized"`
	Tracking     *Tracking      `json:"tracking"`
	Sources      []SourceStatus `json:"sources"`
}

// Tracking describes the synchronization of the local clock, as reported by
// "chronyc tracking".
type Tracking struct {
	ReferenceID    string  `json:"referenceId"`
	Reference      string  `json:"reference"`
	Stratum        int     `json:"stratum"`
	OffsetSeconds  float64 `json:"offsetSeconds"`
	RootDelay      float64 `json:"rootDelaySeconds"`
	RootDispersion float64 `json:"rootDispersionSeconds"`
	UpdateInterval float64 `json:"updateIntervalSeconds"`
	LeapStatus     string  `json:"leapStatus"`
}

// SourceStatus describes an upstream source, as reported by "chronyc sources".
type SourceStatus struct {
	Mode    string `json:"mode"`
	State   string `json:"state"`
	Address string `json:"address"`
	Stratum int    `json:"stratum"`
	// PollSeconds is the polling interval.
	PollSeconds int `json:"pollSeconds"`
	// Reach is the reachability register: 377 (octal) means the last eight
	// requests were answered.
	Reach string `json:"reach"`
	// LastRxSeconds is the age of the last sample, -1 when none was received.
	LastRxSeconds int64   `json:"lastRxSeconds"`
	OffsetSeconds float64 `json:"offsetSeconds"`
	ErrorSeconds  float64 `json:"errorSeconds"`
}

var sourceModes = map[string]string{"^": "server", "=": "peer", "#": "refclock"}

var sourceStates = map[string]string{
	"*": "selected",
	"+": "combined",
	"-": "not-combined",
	"?": "unreachable",
	"x": "falseticker",
	"~": "too-variable",
}

// parseTracking parses the output of "chronyc -c tracking".
func parseTracking(out string) (*Tracking, error) {
	records, err := csv.NewReader(strings.NewReader(out)).ReadAll()
	if err != nil || len(records) != 1 || len(records[0]) < 14 {
		return nil, fmt.Errorf("unexpected tracking output: %q", out)
	}
	f := records[0]
	var p numParser
	t := &Tracking{
		ReferenceID:    f[0],
		Reference:      f[1],
		Stratum:        p.int(f[2]),
		OffsetSeconds:  p.float(f[4]),
		RootDelay:      p.float(f[10]),
		RootDispersion: p.float(f[11]),
		UpdateInterval: p.float(f[12]),
		LeapStatus:     f[13],
	}
	if p.err != nil {
		return nil, fmt.Errorf("unexpected tracking output: %w", p.err)
	}
	return t, nil
}

// synchronized reports whether the tracking data shows a synchronized clock.
func (t *Tracking) synchronized() bool {
	return t != nil && t.ReferenceID != "00000000" && t.Stratum > 0 && t.Stratum < 16 &&
		t.LeapStatus != "Not synchronised"
}

// parseSources parses the output of "chronyc -c sources".
func parseSources(out string) ([]SourceStatus, error) {
	records, err := csv.NewReader(strings.NewReader(out)).ReadAll()
	if err != nil {
		return nil, fmt.Errorf("unexpected sources output: %w", err)
	}
	sources := make([]SourceStatus, 0, len(records))
	for _, f := range records {
		if len(f) < 10 {
			return nil, fmt.Errorf("unexpected sources line: %q", strings.Join(f, ","))
		}
		var p numParser
		poll := p.int(f[4])
		s := SourceStatus{
			Mode:          lookup(sourceModes, f[0]),
			State:         lookup(sourceStates, f[1]),
			Address:       f[2],
			Stratum:       p.int(f[3]),
			Reach:         f[5],
			LastRxSeconds: parseLastRx(f[6]),
			OffsetSeconds: p.float(f[7]),
			ErrorSeconds:  p.float(f[9]),
		}
		if p.err != nil {
			return nil, fmt.Errorf("unexpected sources line: %w", p.err)
		}
		// The poll interval is reported as a power of two.
		if poll >= 0 && poll < 32 {
			s.PollSeconds = 1 << poll
		}
		sources = append(sources, s)
	}
	return sources, nil
}

func parseLastRx(s string) int64 {
	v, err := strconv.ParseInt(s, 10, 64)
	// chronyd reports a huge value when no sample was ever received.
	if err != nil || v < 0 || v >= 1<<31 {
		return -1
	}
	return v
}

func lookup(m map[string]string, key string) string {
	if v, ok := m[key]; ok {
		return v
	}
	return "unknown"
}

// numParser parses numbers and keeps the first error.
type numParser struct{ err error }

func (p *numParser) int(s string) int {
	v, err := strconv.Atoi(s)
	if err != nil && p.err == nil {
		p.err = err
	}
	return v
}

func (p *numParser) float(s string) float64 {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil && p.err == nil {
		p.err = err
	}
	return v
}
