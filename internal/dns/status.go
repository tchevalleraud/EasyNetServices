package dns

import (
	"context"
	"strconv"
	"strings"

	"github.com/tchevalleraud/EasyNetServices/internal/supervisor"
)

// Status is the runtime state of the DNS service.
type Status struct {
	Authoritative supervisor.State `json:"authoritative"`
	Recursor      supervisor.State `json:"recursor"`
	Stats         *RecursorStats   `json:"stats"`
}

// RecursorStats are the counters of the resolver since it started.
type RecursorStats struct {
	Questions     uint64 `json:"questions"`
	CacheHits     uint64 `json:"cacheHits"`
	CacheMisses   uint64 `json:"cacheMisses"`
	CacheEntries  uint64 `json:"cacheEntries"`
	NoError       uint64 `json:"noErrorAnswers"`
	NXDomain      uint64 `json:"nxdomainAnswers"`
	ServFail      uint64 `json:"servfailAnswers"`
	UptimeSeconds uint64 `json:"uptimeSeconds"`
}

var statNames = []string{
	"questions", "cache-hits", "cache-misses", "cache-entries",
	"noerror-answers", "nxdomain-answers", "servfail-answers", "uptime",
}

// parseStats parses the output of "rec_control get" for statNames, one value
// per line in the same order.
func parseStats(out string) (*RecursorStats, bool) {
	lines := strings.Fields(out)
	if len(lines) != len(statNames) {
		return nil, false
	}
	values := make([]uint64, len(lines))
	for i, l := range lines {
		v, err := strconv.ParseUint(l, 10, 64)
		if err != nil {
			return nil, false
		}
		values[i] = v
	}
	return &RecursorStats{
		Questions: values[0], CacheHits: values[1], CacheMisses: values[2], CacheEntries: values[3],
		NoError: values[4], NXDomain: values[5], ServFail: values[6], UptimeSeconds: values[7],
	}, true
}

// Status returns the runtime state of the authoritative server and the resolver.
func (m *Manager) Status(ctx context.Context) (Status, error) {
	status := Status{Authoritative: supervisor.StateUnavailable, Recursor: supervisor.StateUnavailable}
	settings, err := m.Settings(ctx)
	if err != nil {
		return status, err
	}
	if !m.sup.Available() {
		return status, nil
	}

	status.Authoritative = m.state(ctx, authService)
	if !settings.Enabled {
		status.Recursor = supervisor.StateDisabled
		return status, nil
	}
	status.Recursor = m.state(ctx, recursorService)
	if status.Recursor == supervisor.StateRunning {
		if out, err := m.recControl(ctx, append([]string{"get"}, statNames...)...); err == nil {
			if stats, ok := parseStats(out); ok {
				status.Stats = stats
			}
		}
	}
	return status, nil
}

func (m *Manager) state(ctx context.Context, service string) supervisor.State {
	up, err := m.sup.IsUp(ctx, service)
	if err != nil || !up {
		return supervisor.StateStopped
	}
	return supervisor.StateRunning
}
