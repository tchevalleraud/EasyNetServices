package dhcp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/tchevalleraud/EasyNetServices/internal/store"
	"github.com/tchevalleraud/EasyNetServices/internal/supervisor"
	"github.com/tchevalleraud/EasyNetServices/internal/validation"
)

func fieldErrors(t *testing.T, err error) map[string]string {
	t.Helper()
	var errs validation.Errors
	if !errors.As(err, &errs) {
		t.Fatalf("expected validation errors, got %v", err)
	}
	fields := map[string]string{}
	for _, e := range errs {
		fields[e.Field] = e.Message
	}
	return fields
}

func TestSettingsValidation(t *testing.T) {
	s := DefaultSettings()
	if err := s.Validate(); err != nil {
		t.Fatalf("default settings should be valid: %v", err)
	}

	s.DHCPv4.Enabled = true
	s.DHCPv6.PreferredLifetime = s.DHCPv6.ValidLifetime + 1
	s.DHCPv4.Options.DNSServers = []string{"2001:db8::1"}
	s.DHCPv6.Options.DomainName = "home.lan"
	s.DHCPv6.Options.DomainSearch = []string{"bad domain"}
	fields := fieldErrors(t, s.Validate())
	for _, f := range []string{"interfaces", "dhcpv6.preferredLifetime", "dhcpv4.options.dnsServers[0]", "dhcpv6.options.domainName", "dhcpv6.options.domainSearch[0]"} {
		if _, ok := fields[f]; !ok {
			t.Errorf("expected an error on %s, got %v", f, fields)
		}
	}
}

func TestSubnetValidation(t *testing.T) {
	s := Subnet{
		Subnet:  "192.168.1.7/24",
		Pools:   []string{"192.168.1.100-192.168.1.150", "192.168.1.128/26", "10.0.0.1 - 10.0.0.9"},
		Gateway: "192.168.2.1",
		Reservations: []Reservation{
			{HWAddress: "AA-BB-CC-DD-EE-FF", IPAddress: "192.168.1.10", Hostname: "NAS"},
			{HWAddress: "aa:bb:cc:dd:ee:ff", IPAddress: "192.168.1.10"},
			{DUID: "00:01:02", IPAddress: "192.168.1.11"},
		},
		Interface: "eth1",
	}
	s.Normalize(V4)
	if s.Subnet != "192.168.1.0/24" || s.Pools[0] != "192.168.1.100 - 192.168.1.150" {
		t.Fatalf("unexpected normalization: %+v", s)
	}
	if r := s.Reservations[0]; r.HWAddress != "aa:bb:cc:dd:ee:ff" || r.Hostname != "nas" {
		t.Fatalf("unexpected reservation normalization: %+v", r)
	}
	fields := fieldErrors(t, s.Validate(V4, []string{"eth0"}))
	for _, f := range []string{"interface", "pools[1]", "pools[2]", "gateway", "reservations[1].ipAddress", "reservations[1].hwAddress", "reservations[2].duid"} {
		if _, ok := fields[f]; !ok {
			t.Errorf("expected an error on %s, got %v", f, fields)
		}
	}
	if _, ok := fields["reservations[0].ipAddress"]; ok {
		t.Errorf("the first reservation is valid: %v", fields)
	}

	v6 := Subnet{
		Subnet:       "2001:db8:1::/64",
		Interface:    "eth0",
		Pools:        []string{"2001:db8:1::1000 - 2001:db8:1::1fff"},
		Reservations: []Reservation{{DUID: "00:03:00:01:AA:BB:CC:DD:EE:FF", IPAddress: "2001:db8:1::10"}},
	}
	v6.Normalize(V6)
	if err := v6.Validate(V6, []string{"eth0"}); err != nil {
		t.Fatalf("IPv6 subnet should be valid: %v", err)
	}
	v6.Gateway = "2001:db8:1::1"
	if _, ok := fieldErrors(t, v6.Validate(V6, []string{"eth0"}))["gateway"]; !ok {
		t.Error("a DHCPv6 gateway should be refused")
	}
}

func TestRender(t *testing.T) {
	s := DefaultSettings()
	s.Interfaces = []string{"eth0"}
	s.DHCPv4.Options = Options{DNSServers: []string{"192.168.1.2"}, DomainName: "home.lan"}
	subnets := []Subnet{{
		ID: 3, Subnet: "192.168.1.0/24", Pools: []string{"192.168.1.100 - 192.168.1.199"}, Gateway: "192.168.1.1",
		ValidLifetime: 600,
		Options:       Options{NTPServers: []string{"192.168.1.2"}},
		Reservations:  []Reservation{{HWAddress: "aa:bb:cc:dd:ee:ff", IPAddress: "192.168.1.10", Hostname: "nas"}},
	}}
	var conf struct {
		Dhcp4 struct {
			Interfaces struct {
				Interfaces []string `json:"interfaces"`
				SocketType string   `json:"dhcp-socket-type"`
			} `json:"interfaces-config"`
			Leases struct {
				Name string `json:"name"`
			} `json:"lease-database"`
			Options []map[string]string `json:"option-data"`
			Subnets []struct {
				ID           uint32              `json:"id"`
				Lifetime     uint32              `json:"valid-lifetime"`
				Options      []map[string]string `json:"option-data"`
				Reservations []map[string]any    `json:"reservations"`
			} `json:"subnet4"`
		}
	}
	out := render(renderInput{Family: V4, Settings: s, Subnets: subnets, DataDir: "/data/kea"})
	if err := json.Unmarshal(out, &conf); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	d := conf.Dhcp4
	if d.Interfaces.Interfaces[0] != "eth0" || d.Interfaces.SocketType != "raw" || d.Leases.Name != "/data/kea/kea-leases4.csv" {
		t.Errorf("unexpected configuration: %s", out)
	}
	if len(d.Options) != 2 || d.Options[0]["name"] != "domain-name-servers" || d.Options[1]["data"] != "home.lan" {
		t.Errorf("unexpected global options: %v", d.Options)
	}
	sub := d.Subnets[0]
	if sub.ID != 3 || sub.Lifetime != 600 || len(sub.Options) != 2 || sub.Options[0]["name"] != "routers" || sub.Options[1]["name"] != "ntp-servers" {
		t.Errorf("unexpected subnet: %+v", sub)
	}
	if sub.Reservations[0]["hw-address"] != "aa:bb:cc:dd:ee:ff" || sub.Reservations[0]["hostname"] != "nas" {
		t.Errorf("unexpected reservation: %v", sub.Reservations)
	}

	v6 := render(renderInput{Family: V6, Settings: s, Subnets: []Subnet{{
		ID: 1, Subnet: "2001:db8::/64", Interface: "eth0", ValidLifetime: 600,
		Reservations: []Reservation{{DUID: "00:01:02", IPAddress: "2001:db8::10"}},
	}}, DataDir: "/data/kea"})
	for _, want := range []string{`"Dhcp6"`, `"preferred-lifetime": 600`, `"interface": "eth0"`, `"ip-addresses": [`, `"kea6-ctrl-socket"`} {
		if !strings.Contains(string(v6), want) {
			t.Errorf("DHCPv6 configuration misses %s:\n%s", want, v6)
		}
	}
	if strings.Contains(string(v6), "dhcp-socket-type") {
		t.Error("DHCPv6 has no socket type")
	}
}

func TestKeaErrorMessage(t *testing.T) {
	out := "2026-10-12 06:35:00.529 ERROR [kea-dhcp4.dhcp4/1.2] DHCP4_PARSER_FAIL failed\n" +
		"Error encountered: subnet configuration failed: bad pool (/data/kea/kea-dhcp4.conf.test:7:14)"
	match := keaErrorRe.FindStringSubmatch(out)
	if match == nil || match[1] != "subnet configuration failed: bad pool" {
		t.Fatalf("unexpected match: %q", match)
	}
}

// fakeKea answers the commands of the control channel on a unix socket.
type fakeKea struct {
	mu       sync.Mutex
	commands []string
	leases   []map[string]any
}

func (k *fakeKea) serve(t *testing.T, path string) {
	l, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			go k.handle(conn)
		}
	}()
}

func (k *fakeKea) handle(conn net.Conn) {
	defer conn.Close()
	var req struct {
		Command   string         `json:"command"`
		Arguments map[string]any `json:"arguments"`
	}
	if err := json.NewDecoder(conn).Decode(&req); err != nil {
		return
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	k.commands = append(k.commands, req.Command)
	var resp any
	switch req.Command {
	case "config-reload":
		resp = map[string]any{"result": 0}
	case "lease4-get-all":
		if len(k.leases) == 0 {
			resp = map[string]any{"result": 3, "arguments": map[string]any{"leases": []any{}}}
		} else {
			resp = map[string]any{"result": 0, "arguments": map[string]any{"leases": k.leases}}
		}
	case "lease4-del":
		resp = map[string]any{"result": 3, "text": "IPv4 lease not found."}
		for i, l := range k.leases {
			if l["ip-address"] == req.Arguments["ip-address"] {
				k.leases = append(k.leases[:i], k.leases[i+1:]...)
				resp = map[string]any{"result": 0}
				break
			}
		}
	case "status-get":
		resp = map[string]any{"result": 0, "arguments": map[string]any{
			"sockets": map[string]any{"status": "retrying", "errors": []string{"the interface eth0 is down"}},
		}}
	case "statistic-get-all":
		resp = map[string]any{"result": 0, "arguments": map[string]any{
			"pkt4-received":                [][]any{{12, "2026-10-12 06:34:42.011652"}},
			"pkt4-sent":                    [][]any{{10, "2026-10-12 06:34:42.011652"}},
			"subnet[1].total-addresses":    [][]any{{100, "2026-10-12 06:34:42.011652"}},
			"subnet[1].assigned-addresses": [][]any{{2, "2026-10-12 06:34:52"}, {1, "2026-10-12 06:34:50"}},
		}}
	default:
		resp = map[string]any{"result": 2, "text": "unsupported command"}
	}
	json.NewEncoder(conn).Encode(resp)
}

func (k *fakeKea) sent(command string) bool {
	k.mu.Lock()
	defer k.mu.Unlock()
	for _, c := range k.commands {
		if c == command {
			return true
		}
	}
	return false
}

type fakeSupervisor struct{ up map[string]bool }

func (f *fakeSupervisor) Available() bool { return true }
func (f *fakeSupervisor) Up(_ context.Context, n string) error {
	f.up[n] = true
	return nil
}
func (f *fakeSupervisor) Down(_ context.Context, n string) error {
	f.up[n] = false
	return nil
}
func (f *fakeSupervisor) Restart(_ context.Context, n string) error {
	f.up[n] = true
	return nil
}
func (f *fakeSupervisor) IsUp(_ context.Context, n string) (bool, error) { return f.up[n], nil }

func newTestManager(t *testing.T) (*Manager, *fakeSupervisor, *fakeKea, *[]string) {
	t.Helper()
	ctx := context.Background()
	dir := t.TempDir()
	st, err := store.Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	// Unix socket paths are limited in length: keep them short.
	sockets, err := os.MkdirTemp("", "kea")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(sockets) })

	sup := &fakeSupervisor{up: map[string]bool{}}
	m := NewManager(st, sup, dir, slog.New(slog.NewTextHandler(io.Discard, nil)))
	m.SetSocketDir(sockets)
	m.SetInterfaceLister(func() ([]NetInterface, error) {
		return []NetInterface{{Name: "lo", IsLoopback: true}, {Name: "eth0", Up: true}}, nil
	})
	var tested []string
	m.SetRunner(func(_ context.Context, env []string, name string, args ...string) ([]byte, error) {
		data, _ := os.ReadFile(args[1])
		if strings.Contains(string(data), "10.66.0.0") {
			return []byte("Error encountered: refused by Kea (" + args[1] + ":3:4)"), errors.New("exit status 1")
		}
		tested = append(tested, name)
		return nil, nil
	})
	kea := &fakeKea{}
	kea.serve(t, filepath.Join(sockets, socketName(V4)))
	if err := m.Init(ctx); err != nil {
		t.Fatal(err)
	}
	return m, sup, kea, &tested
}

func TestManager(t *testing.T) {
	ctx := context.Background()
	m, sup, kea, tested := newTestManager(t)

	if sup.up["kea-dhcp4"] || sup.up["kea-dhcp6"] {
		t.Fatal("servers should be stopped by default")
	}
	if _, err := os.Stat(m.confPath(V4)); err != nil {
		t.Fatalf("configuration should be written at start: %v", err)
	}
	if ifaces, _ := m.Interfaces(); len(ifaces) != 1 || ifaces[0].Name != "eth0" {
		t.Fatalf("loopback interfaces should be hidden: %v", ifaces)
	}

	s := DefaultSettings()
	s.Interfaces = []string{"eth1"}
	s.DHCPv4.Enabled = true
	if _, ok := fieldErrors(t, func() error { _, err := m.UpdateSettings(ctx, s); return err }())["interfaces[0]"]; !ok {
		t.Fatal("an unknown interface should be refused")
	}
	s.Interfaces = []string{"eth0"}
	if _, err := m.UpdateSettings(ctx, s); err != nil {
		t.Fatal(err)
	}
	if !sup.up["kea-dhcp4"] || sup.up["kea-dhcp6"] {
		t.Fatalf("only DHCPv4 should run: %v", sup.up)
	}
	if len(*tested) != 2 {
		t.Fatalf("both configurations should be tested by Kea, got %v", *tested)
	}

	created, err := m.CreateSubnet(ctx, V4, Subnet{Subnet: "192.168.1.0/24", Interface: "eth0", Pools: []string{"192.168.1.100-192.168.1.199"}})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID != 1 || !kea.sent("config-reload") {
		t.Fatalf("expected subnet 1 and a reload, got %+v %v", created, kea.commands)
	}
	second, err := m.CreateSubnet(ctx, V4, Subnet{Subnet: "192.168.2.0/24"})
	if err != nil || second.ID != 2 {
		t.Fatalf("expected subnet 2, got %+v %v", second, err)
	}
	if _, err := m.CreateSubnet(ctx, V4, Subnet{Subnet: "192.168.2.0/25"}); err == nil {
		t.Fatal("an overlapping subnet should be refused")
	}
	var rejected *RejectedError
	if _, err := m.CreateSubnet(ctx, V4, Subnet{Subnet: "10.66.0.0/16"}); !errors.As(err, &rejected) || rejected.Message != "DHCPv4 refused the configuration: refused by Kea" {
		t.Fatalf("expected Kea to refuse the subnet, got %v", err)
	}
	if list, _ := m.Subnets(ctx, V4); len(list) != 2 {
		t.Fatalf("a refused subnet must not be stored: %+v", list)
	}

	// The interface of a subnet cannot be removed from the settings.
	s.Interfaces = []string{}
	s.DHCPv4.Enabled = false
	if _, ok := fieldErrors(t, func() error { _, err := m.UpdateSettings(ctx, s); return err }())["interfaces"]; !ok {
		t.Fatal("removing an interface used by a subnet should be refused")
	}

	updated, err := m.UpdateSubnet(ctx, V4, 1, Subnet{ID: 9, Subnet: "192.168.1.0/24", Description: "LAN",
		Reservations: []Reservation{{HWAddress: "aa:bb:cc:dd:ee:ff", IPAddress: "192.168.1.10"}}})
	if err != nil || updated.ID != 1 || updated.Description != "LAN" {
		t.Fatalf("unexpected update: %+v %v", updated, err)
	}
	if _, err := m.UpdateSubnet(ctx, V4, 7, updated); !errors.Is(err, ErrSubnetNotFound) {
		t.Fatalf("expected ErrSubnetNotFound, got %v", err)
	}

	// Leases and status.
	if leases, err := m.Leases(ctx, V4, 0); err != nil || len(leases) != 0 {
		t.Fatalf("expected no lease, got %v %v", leases, err)
	}
	kea.leases = []map[string]any{
		{"ip-address": "192.168.1.120", "hw-address": "aa:bb:cc:dd:ee:01", "hostname": "laptop", "subnet-id": 1, "state": 0, "cltt": 1000, "valid-lft": 3600},
		{"ip-address": "192.168.1.10", "hw-address": "aa:bb:cc:dd:ee:ff", "subnet-id": 1, "state": 1, "cltt": 1000, "valid-lft": 3600},
	}
	leases, err := m.Leases(ctx, V4, 1)
	if err != nil || len(leases) != 2 {
		t.Fatalf("expected two leases, got %v %v", leases, err)
	}
	if leases[0].Address != "192.168.1.10" || !leases[0].Reserved || leases[0].State != "declined" || leases[1].Reserved {
		t.Fatalf("unexpected leases: %+v", leases)
	}
	if leases[1].Expires.Unix() != 4600 {
		t.Fatalf("unexpected expiry: %v", leases[1].Expires)
	}
	if err := m.DeleteLease(ctx, V4, "192.168.1.120"); err != nil {
		t.Fatal(err)
	}
	if err := m.DeleteLease(ctx, V4, "192.168.1.120"); !errors.Is(err, ErrLeaseNotFound) {
		t.Fatalf("expected ErrLeaseNotFound, got %v", err)
	}
	if _, err := m.Leases(ctx, V6, 0); !errors.Is(err, ErrNotRunning) {
		t.Fatalf("expected ErrNotRunning for DHCPv6, got %v", err)
	}

	status, err := m.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	v4 := status.DHCPv4
	if v4.State != supervisor.StateRunning || v4.PacketsReceived != 12 || len(v4.SocketErrors) != 1 {
		t.Fatalf("unexpected DHCPv4 status: %+v", v4)
	}
	if u := v4.Subnets[0]; u.ID != 1 || u.Total != 100 || u.Assigned != 2 {
		t.Fatalf("unexpected usage: %+v", u)
	}
	if status.DHCPv6.State != supervisor.StateDisabled {
		t.Fatalf("DHCPv6 should be disabled: %+v", status.DHCPv6)
	}

	if err := m.DeleteSubnet(ctx, V4, 2); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Subnet(ctx, V4, 2); !errors.Is(err, ErrSubnetNotFound) {
		t.Fatalf("expected the subnet to be deleted, got %v", err)
	}
}
