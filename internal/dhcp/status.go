package dhcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/tchevalleraud/EasyNetServices/internal/supervisor"
)

// Status is the runtime state of the DHCP servers.
type Status struct {
	DHCPv4 FamilyStatus `json:"dhcpv4"`
	DHCPv6 FamilyStatus `json:"dhcpv6"`
}

// FamilyStatus is the runtime state of the DHCPv4 or DHCPv6 server.
type FamilyStatus struct {
	State supervisor.State `json:"state"`
	// SocketErrors explain why the server cannot listen on an interface.
	SocketErrors []string      `json:"socketErrors"`
	Subnets      []SubnetUsage `json:"subnets"`
	// PacketsReceived and PacketsSent count the DHCP messages since start.
	PacketsReceived float64 `json:"packetsReceived"`
	PacketsSent     float64 `json:"packetsSent"`
}

// SubnetUsage is the address usage of a subnet. Counts are floating-point
// numbers because the size of an IPv6 pool exceeds 64-bit integers.
type SubnetUsage struct {
	ID       uint32  `json:"id"`
	Subnet   string  `json:"subnet"`
	Total    float64 `json:"total"`
	Assigned float64 `json:"assigned"`
	Declined float64 `json:"declined"`
}

// Status returns the runtime state of both servers.
func (m *Manager) Status(ctx context.Context) (Status, error) {
	settings, err := m.Settings(ctx)
	if err != nil {
		return Status{}, err
	}
	var status Status
	for _, f := range []Family{V4, V6} {
		fs, err := m.familyStatus(ctx, f, settings.Family(f).Enabled)
		if err != nil {
			return Status{}, err
		}
		if f == V4 {
			status.DHCPv4 = fs
		} else {
			status.DHCPv6 = fs
		}
	}
	return status, nil
}

func (m *Manager) familyStatus(ctx context.Context, f Family, enabled bool) (FamilyStatus, error) {
	fs := FamilyStatus{State: supervisor.StateUnavailable, SocketErrors: []string{}, Subnets: []SubnetUsage{}}
	subnets, err := m.Subnets(ctx, f)
	if err != nil {
		return fs, err
	}
	for _, s := range subnets {
		fs.Subnets = append(fs.Subnets, SubnetUsage{ID: s.ID, Subnet: s.Subnet})
	}
	switch {
	case !m.sup.Available():
		return fs, nil
	case !enabled:
		fs.State = supervisor.StateDisabled
		return fs, nil
	case !m.running(ctx, f):
		fs.State = supervisor.StateStopped
		return fs, nil
	}
	fs.State = supervisor.StateRunning

	client := m.client(f)
	var st struct {
		Sockets struct {
			Status string   `json:"status"`
			Errors []string `json:"errors"`
		} `json:"sockets"`
	}
	if err := client.command(ctx, "status-get", nil, &st); err == nil && st.Sockets.Status != "ready" {
		fs.SocketErrors = append(fs.SocketErrors, st.Sockets.Errors...)
	}

	var stats map[string]json.RawMessage
	if err := client.command(ctx, "statistic-get-all", nil, &stats); err != nil {
		m.log.Debug("cannot read Kea statistics", "family", f, "error", err)
		return fs, nil
	}
	total, assigned := "total-addresses", "assigned-addresses"
	received, sent := "pkt4-received", "pkt4-sent"
	if f == V6 {
		total, assigned = "total-nas", "assigned-nas"
		received, sent = "pkt6-received", "pkt6-sent"
	}
	fs.PacketsReceived = statValue(stats[received])
	fs.PacketsSent = statValue(stats[sent])
	for i := range fs.Subnets {
		key := fmt.Sprintf("subnet[%d].", fs.Subnets[i].ID)
		fs.Subnets[i].Total = statValue(stats[key+total])
		fs.Subnets[i].Assigned = statValue(stats[key+assigned])
		fs.Subnets[i].Declined = statValue(stats[key+"declined-addresses"])
	}
	return fs, nil
}

// statValue returns the latest value of a Kea statistic, a list of
// [value, timestamp] samples, newest first.
func statValue(raw json.RawMessage) float64 {
	var samples [][]json.RawMessage
	if err := json.Unmarshal(raw, &samples); err != nil || len(samples) == 0 || len(samples[0]) == 0 {
		return 0
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(string(samples[0][0])), 64)
	if err != nil {
		return 0
	}
	return v
}
