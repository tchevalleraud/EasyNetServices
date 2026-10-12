package dhcp

import (
	"context"
	"errors"
	"net/netip"
	"slices"
	"time"
)

// Lease is an address assigned to a client.
type Lease struct {
	Address   string `json:"address"`
	HWAddress string `json:"hwAddress"`
	// DUID identifies a DHCPv6 client.
	DUID     string    `json:"duid"`
	Hostname string    `json:"hostname"`
	SubnetID uint32    `json:"subnetId"`
	State    string    `json:"state"`
	Expires  time.Time `json:"expires"`
	// Reserved is set when the address matches a reservation of the client.
	Reserved bool `json:"reserved"`
}

type keaLease struct {
	IPAddress string `json:"ip-address"`
	HWAddress string `json:"hw-address"`
	DUID      string `json:"duid"`
	Hostname  string `json:"hostname"`
	SubnetID  uint32 `json:"subnet-id"`
	State     int    `json:"state"`
	CLTT      int64  `json:"cltt"`
	ValidLft  int64  `json:"valid-lft"`
	Type      string `json:"type"`
}

var leaseStates = map[int]string{0: "active", 1: "declined", 2: "expired", 3: "released", 4: "registered"}

// Leases returns the leases of a family, optionally of a single subnet.
func (m *Manager) Leases(ctx context.Context, f Family, subnetID uint32) ([]Lease, error) {
	if !m.running(ctx, f) {
		return nil, ErrNotRunning
	}
	var args any
	if subnetID != 0 {
		args = map[string]any{"subnets": []uint32{subnetID}}
	}
	var resp struct {
		Leases []keaLease `json:"leases"`
	}
	err := m.client(f).command(ctx, "lease"+string(f[1])+"-get-all", args, &resp)
	if err != nil && !errors.Is(err, errKeaEmpty) {
		return nil, err
	}

	subnets, err := m.Subnets(ctx, f)
	if err != nil {
		return nil, err
	}
	reserved := map[string]bool{}
	for _, s := range subnets {
		for _, r := range s.Reservations {
			reserved[r.IPAddress+"|"+r.HWAddress+"|"+r.DUID] = true
		}
	}

	leases := make([]Lease, 0, len(resp.Leases))
	for _, l := range resp.Leases {
		// Delegated prefixes are not managed yet.
		if l.Type != "" && l.Type != "IA_NA" {
			continue
		}
		state, ok := leaseStates[l.State]
		if !ok {
			state = "unknown"
		}
		leases = append(leases, Lease{
			Address:   l.IPAddress,
			HWAddress: l.HWAddress,
			DUID:      l.DUID,
			Hostname:  l.Hostname,
			SubnetID:  l.SubnetID,
			State:     state,
			Expires:   time.Unix(l.CLTT+l.ValidLft, 0).UTC(),
			Reserved:  reserved[l.IPAddress+"|"+l.HWAddress+"|"] || reserved[l.IPAddress+"||"+l.DUID],
		})
	}
	slices.SortFunc(leases, func(a, b Lease) int {
		x, _ := netip.ParseAddr(a.Address)
		y, _ := netip.ParseAddr(b.Address)
		return x.Compare(y)
	})
	return leases, nil
}

// DeleteLease releases a lease, so that its address can be assigned again.
func (m *Manager) DeleteLease(ctx context.Context, f Family, address string) error {
	if _, ok := parseAddr(address, f); !ok {
		return ErrLeaseNotFound
	}
	if !m.running(ctx, f) {
		return ErrNotRunning
	}
	err := m.client(f).command(ctx, "lease"+string(f[1])+"-del", map[string]any{"ip-address": address}, nil)
	if errors.Is(err, errKeaEmpty) {
		return ErrLeaseNotFound
	}
	if err == nil {
		m.log.Info("DHCP lease deleted", "family", f, "address", address)
	}
	return err
}
