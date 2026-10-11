package dns

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/tchevalleraud/EasyNetServices/internal/store"
	"github.com/tchevalleraud/EasyNetServices/internal/supervisor"
	"github.com/tchevalleraud/EasyNetServices/internal/validation"
)

const (
	settingsKey     = "dns"
	authService     = "pdns-auth"
	recursorService = "pdns-recursor"
	socketDir       = "/run/pdns-recursor"
)

// ErrApply is returned when a change was saved but could not be applied.
var ErrApply = errors.New("change saved but could not be applied")

// Runner executes a command and returns its combined output.
type Runner func(ctx context.Context, name string, args ...string) ([]byte, error)

func execRunner(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

// Prober checks that a listen address is free before the resolver binds it.
type Prober func(address string) error

func probePort53(address string) error {
	hostPort := net.JoinHostPort(address, "53")
	udp, err := net.ListenPacket("udp", hostPort)
	if err != nil {
		return err
	}
	udp.Close()
	tcp, err := net.Listen("tcp", hostPort)
	if err != nil {
		return err
	}
	return tcp.Close()
}

// Manager stores the DNS configuration and applies it to PowerDNS.
type Manager struct {
	store        *store.Store
	sup          supervisor.Supervisor
	run          Runner
	probe        Prober
	log          *slog.Logger
	dir          string
	authConf     string
	recursorConf string
	apiKeyFile   string
	pdns         *pdnsClient
	ipv6         bool
	mu           sync.Mutex
}

// NewManager returns a manager writing the PowerDNS files under dataDir/pdns.
func NewManager(st *store.Store, sup supervisor.Supervisor, dataDir string, log *slog.Logger) *Manager {
	dir := filepath.Join(dataDir, "pdns")
	_, err := os.Stat("/proc/net/if_inet6")
	return &Manager{
		store:        st,
		sup:          sup,
		run:          execRunner,
		probe:        probePort53,
		log:          log,
		dir:          dir,
		authConf:     filepath.Join(dir, "pdns.conf"),
		recursorConf: filepath.Join(dir, "recursor.yml"),
		apiKeyFile:   filepath.Join(dir, "api.key"),
		ipv6:         err == nil,
	}
}

// SetRunner and SetProber replace the system interactions, for tests.
func (m *Manager) SetRunner(r Runner) { m.run = r }
func (m *Manager) SetProber(p Prober) { m.probe = p }

// SetAuthURL points the manager to another authoritative server API, for tests.
func (m *Manager) SetAuthURL(url string) {
	if m.pdns != nil {
		m.pdns.baseURL = url
	}
}

// Init writes the authoritative server configuration and starts it. It must
// be called before the other methods.
func (m *Manager) Init(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.initAuth(ctx)
}

// StartResolver waits for the authoritative server, which provides the list of
// local zones, then applies the stored resolver settings. It may take a while
// and should run in the background.
func (m *Manager) StartResolver(ctx context.Context) error {
	if m.sup.Available() {
		m.waitForAuth(ctx, time.Minute)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	settings, err := m.Settings(ctx)
	if err != nil {
		return err
	}
	return m.applyRecursor(ctx, settings, false)
}

// initAuth writes the authoritative server configuration and starts it.
func (m *Manager) initAuth(ctx context.Context) error {
	if err := os.MkdirAll(m.dir, 0o755); err != nil {
		return err
	}
	key, err := m.apiKey()
	if err != nil {
		return err
	}
	m.pdns = newPDNSClient("http://"+authAPIAddress, key)

	changed, err := writeFile(m.authConf, renderAuth(m.dir, key), 0o640)
	if err != nil {
		return err
	}
	// The configuration holds the API key: readable by the pdns group only.
	if g, err := user.LookupGroup("pdns"); err == nil {
		if gid, err := strconv.Atoi(g.Gid); err == nil {
			os.Chown(m.authConf, 0, gid)
		}
	}
	if !m.sup.Available() {
		m.log.Warn("service supervision unavailable, PowerDNS is not managed")
		return nil
	}
	if changed {
		return m.sup.Restart(ctx, authService)
	}
	return m.sup.Up(ctx, authService)
}

// apiKey returns the key protecting the authoritative server API, generating
// it on first use.
func (m *Manager) apiKey() (string, error) {
	if data, err := os.ReadFile(m.apiKeyFile); err == nil && len(bytes.TrimSpace(data)) > 0 {
		return string(bytes.TrimSpace(data)), nil
	}
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	key := hex.EncodeToString(b)
	if err := os.WriteFile(m.apiKeyFile, []byte(key+"\n"), 0o600); err != nil {
		return "", err
	}
	return key, nil
}

func (m *Manager) waitForAuth(ctx context.Context, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := m.pdns.listZones(ctx); err == nil {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second):
		}
	}
	m.log.Warn("authoritative server did not start in time")
}

// Settings returns the stored resolver settings, or the defaults.
func (m *Manager) Settings(ctx context.Context) (Settings, error) {
	var s Settings
	err := m.store.GetSetting(ctx, settingsKey, &s)
	if errors.Is(err, store.ErrNotFound) {
		s = DefaultSettings()
	} else if err != nil {
		return Settings{}, err
	}
	s.Normalize()
	return s, nil
}

// UpdateSettings validates, stores and applies resolver settings. It returns
// validation.Errors when they are invalid, and ErrApply when they were stored
// but could not be applied.
func (m *Manager) UpdateSettings(ctx context.Context, s Settings) (Settings, error) {
	s.Normalize()
	if err := s.Validate(); err != nil {
		return Settings{}, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.checkListen(ctx, s); err != nil {
		return Settings{}, err
	}
	if err := m.store.PutSetting(ctx, settingsKey, s); err != nil {
		return Settings{}, err
	}
	if err := m.applyRecursor(ctx, s, false); err != nil {
		m.log.Error("failed to apply DNS settings", "error", err)
		return s, fmt.Errorf("%w: %v", ErrApply, err)
	}
	m.log.Info("DNS settings applied", "enabled", s.Enabled, "resolution", s.Resolution)
	return s, nil
}

// checkListen makes sure port 53 is free on the listen addresses when the
// resolver is about to start, so that a conflict is reported to the operator
// instead of making the resolver fail in a loop.
func (m *Manager) checkListen(ctx context.Context, s Settings) error {
	if !s.Enabled || !m.sup.Available() {
		return nil
	}
	if up, err := m.sup.IsUp(ctx, recursorService); err == nil && up {
		// The resolver already holds the port.
		return nil
	}
	addresses := s.ListenAddresses
	field := "listenAddresses"
	if len(addresses) == 0 {
		addresses = []string{"0.0.0.0"}
		if m.ipv6 {
			addresses = append(addresses, "::")
		}
	}
	var errs validation.Errors
	for i, a := range addresses {
		if err := m.probe(a); err != nil {
			f := field
			if len(s.ListenAddresses) > 0 {
				f = fmt.Sprintf("%s[%d]", field, i)
			}
			display := a
			if strings.Contains(a, ":") {
				display = "[" + a + "]"
			}
			errs.Add(f, "port 53 is not available on %s: %v", display, unwrapNetError(err))
		}
	}
	return errs.Err()
}

func unwrapNetError(err error) error {
	var opErr *net.OpError
	if errors.As(err, &opErr) && opErr.Err != nil {
		return opErr.Err
	}
	return err
}

// applyRecursor writes the recursor configuration and brings it to the wanted
// state. With zonesOnly, only the zone list changed: the recursor reloads its
// zones instead of restarting, which keeps its cache.
func (m *Manager) applyRecursor(ctx context.Context, s Settings, zonesOnly bool) error {
	zones, err := m.localZoneNames(ctx)
	if err != nil {
		m.log.Warn("cannot list local zones, they will not be resolvable", "error", err)
	}
	// Stop a disabled resolver before writing its configuration, so that it
	// does not start on it in the meantime.
	if !s.Enabled && m.sup.Available() {
		if err := m.sup.Down(ctx, recursorService); err != nil {
			return err
		}
	}
	content := renderRecursor(recursorInput{Settings: s, LocalZones: zones, IPv6: m.ipv6, SocketDir: socketDir})
	changed, err := writeFile(m.recursorConf, content, 0o644)
	if err != nil {
		return err
	}
	if !m.sup.Available() || !s.Enabled {
		return nil
	}
	up, _ := m.sup.IsUp(ctx, recursorService)
	switch {
	case !changed && up:
		return nil
	case zonesOnly && up:
		if out, err := m.recControl(ctx, "reload-zones"); err != nil {
			return fmt.Errorf("reload zones: %v: %s", err, out)
		}
		return nil
	case changed:
		return m.sup.Restart(ctx, recursorService)
	default:
		return m.sup.Up(ctx, recursorService)
	}
}

func (m *Manager) localZoneNames(ctx context.Context) ([]string, error) {
	if m.pdns == nil {
		return nil, ErrAuthUnavailable
	}
	zones, err := m.pdns.listZones(ctx)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(zones))
	for _, z := range zones {
		names = append(names, CanonicalName(z.Name))
	}
	return names, nil
}

func (m *Manager) recControl(ctx context.Context, args ...string) (string, error) {
	out, err := m.run(ctx, "rec_control", append([]string{"--socket-dir=" + socketDir}, args...)...)
	return strings.TrimSpace(string(out)), err
}

// recControlIfUp runs a rec_control command when the resolver is running.
// Failures are logged: the change is applied anyway at the next restart.
func (m *Manager) recControlIfUp(ctx context.Context, args ...string) {
	if !m.sup.Available() {
		return
	}
	if up, err := m.sup.IsUp(ctx, recursorService); err != nil || !up {
		return
	}
	if out, err := m.recControl(ctx, args...); err != nil {
		m.log.Warn("rec_control command failed", "command", args[0], "error", err, "output", out)
	}
}

// wipeCache removes the cached answers of a zone after its records changed.
func (m *Manager) wipeCache(ctx context.Context, zone string) {
	m.recControlIfUp(ctx, "wipe-cache", fqdn(zone)+"$")
}

// Zones lists the local zones.
func (m *Manager) Zones(ctx context.Context) ([]Zone, error) {
	if m.pdns == nil {
		return nil, ErrAuthUnavailable
	}
	list, err := m.pdns.listZones(ctx)
	if err != nil {
		return nil, err
	}
	zones := make([]Zone, 0, len(list))
	for _, z := range list {
		zones = append(zones, Zone{Name: CanonicalName(z.Name), Kind: z.Kind, Serial: z.Serial})
	}
	return zones, nil
}

// Zone returns a local zone with its records.
func (m *Manager) Zone(ctx context.Context, name string) (*ZoneDetail, error) {
	if m.pdns == nil {
		return nil, ErrAuthUnavailable
	}
	z, err := m.pdns.getZone(ctx, CanonicalName(name))
	if err != nil {
		return nil, err
	}
	detail := &ZoneDetail{
		Zone:   Zone{Name: CanonicalName(z.Name), Kind: z.Kind, Serial: z.Serial},
		RRSets: make([]RRSet, 0, len(z.RRSets)),
	}
	for _, rr := range z.RRSets {
		records := make([]Record, 0, len(rr.Records))
		for _, r := range rr.Records {
			records = append(records, Record{Content: r.Content, Disabled: r.Disabled})
		}
		detail.RRSets = append(detail.RRSets, RRSet{Name: CanonicalName(rr.Name), Type: rr.Type, TTL: rr.TTL, Records: records})
	}
	return detail, nil
}

// CreateZone creates a local zone and makes the resolver answer for it.
func (m *Manager) CreateZone(ctx context.Context, req NewZone) (*ZoneDetail, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	if m.pdns == nil {
		return nil, ErrAuthUnavailable
	}
	nameservers := make([]string, len(req.Nameservers))
	for i, ns := range req.Nameservers {
		nameservers[i] = fqdn(ns)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.pdns.createZone(ctx, pdnsZone{Name: fqdn(req.Name), Kind: "Native", Nameservers: nameservers}); err != nil {
		return nil, err
	}
	m.log.Info("zone created", "zone", req.Name)
	if err := m.reloadZones(ctx); err != nil {
		return nil, err
	}
	// Reloading the zones does not reload the negative trust anchors.
	m.recControlIfUp(ctx, "add-nta", req.Name, "Local zone")
	m.wipeCache(ctx, req.Name)
	return m.Zone(ctx, req.Name)
}

// DeleteZone deletes a local zone.
func (m *Manager) DeleteZone(ctx context.Context, name string) error {
	name = CanonicalName(name)
	if m.pdns == nil {
		return ErrAuthUnavailable
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.pdns.deleteZone(ctx, name); err != nil {
		return err
	}
	m.log.Info("zone deleted", "zone", name)
	if err := m.reloadZones(ctx); err != nil {
		return err
	}
	settings, err := m.Settings(ctx)
	if err != nil {
		return err
	}
	// Keep the trust anchor when a forward zone with the same name takes over.
	forwarded := false
	for _, z := range settings.ForwardZones {
		forwarded = forwarded || z.Name == name
	}
	if !forwarded {
		m.recControlIfUp(ctx, "clear-nta", name)
	}
	m.wipeCache(ctx, name)
	return nil
}

func (m *Manager) reloadZones(ctx context.Context) error {
	s, err := m.Settings(ctx)
	if err != nil {
		return err
	}
	if err := m.applyRecursor(ctx, s, true); err != nil {
		return fmt.Errorf("%w: %v", ErrApply, err)
	}
	return nil
}

// ReplaceRRSet creates or replaces an RRSet of a zone.
func (m *Manager) ReplaceRRSet(ctx context.Context, zone string, rr RRSet) (RRSet, error) {
	zone = CanonicalName(zone)
	if err := rr.Validate(zone); err != nil {
		return RRSet{}, err
	}
	if m.pdns == nil {
		return RRSet{}, ErrAuthUnavailable
	}
	records := make([]pdnsRecord, len(rr.Records))
	for i, r := range rr.Records {
		records[i] = pdnsRecord{Content: r.Content, Disabled: r.Disabled}
	}
	err := m.pdns.patchRRSets(ctx, zone, []pdnsRRSet{{
		Name: fqdn(rr.Name), Type: rr.Type, TTL: rr.TTL, ChangeType: "REPLACE", Records: records,
	}})
	if err != nil {
		return RRSet{}, err
	}
	m.wipeCache(ctx, zone)
	return rr, nil
}

// DeleteRRSet deletes an RRSet of a zone.
func (m *Manager) DeleteRRSet(ctx context.Context, zone, name, rrtype string) error {
	zone = CanonicalName(zone)
	name = CanonicalName(name)
	rrtype = strings.ToUpper(rrtype)
	if !InZone(name, zone) || !isEditableType(rrtype) {
		return ErrZoneNotFound
	}
	if m.pdns == nil {
		return ErrAuthUnavailable
	}
	err := m.pdns.patchRRSets(ctx, zone, []pdnsRRSet{{
		Name: fqdn(name), Type: rrtype, ChangeType: "DELETE", Records: []pdnsRecord{},
	}})
	if err != nil {
		return err
	}
	m.wipeCache(ctx, zone)
	return nil
}

// writeFile atomically replaces a file and reports whether its content changed.
func writeFile(path, content string, mode os.FileMode) (bool, error) {
	if current, err := os.ReadFile(path); err == nil && string(current) == content {
		return false, nil
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), mode); err != nil {
		return false, err
	}
	return true, os.Rename(tmp, path)
}
