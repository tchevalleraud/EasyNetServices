package dhcp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/tchevalleraud/EasyNetServices/internal/store"
	"github.com/tchevalleraud/EasyNetServices/internal/supervisor"
	"github.com/tchevalleraud/EasyNetServices/internal/validation"
)

const (
	settingsKey = "dhcp"
	// DefaultSocketDir is the directory of the Kea control sockets, which Kea
	// only accepts in KEA_CONTROL_SOCKET_DIR.
	DefaultSocketDir = "/run/kea"
	keaUser          = "_kea"
)

var (
	// ErrApply is returned when a change was saved but could not be applied.
	ErrApply = errors.New("change saved but could not be applied")
	// ErrSubnetNotFound is returned for an unknown subnet.
	ErrSubnetNotFound = errors.New("subnet not found")
	// ErrLeaseNotFound is returned for an unknown lease.
	ErrLeaseNotFound = errors.New("lease not found")
	// ErrNotRunning is returned when the server of a family is not running.
	ErrNotRunning = errors.New("DHCP server not running")
)

// RejectedError is returned when Kea refuses a configuration.
type RejectedError struct {
	Message string
}

func (e *RejectedError) Error() string { return e.Message }

// Runner executes a command with additional environment variables and
// returns its combined output.
type Runner func(ctx context.Context, env []string, name string, args ...string) ([]byte, error)

func execRunner(ctx context.Context, env []string, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = append(os.Environ(), env...)
	return cmd.CombinedOutput()
}

// InterfaceLister returns the network interfaces of the host.
type InterfaceLister func() ([]NetInterface, error)

// Manager stores the DHCP configuration and applies it to Kea.
type Manager struct {
	store      *store.Store
	sup        supervisor.Supervisor
	run        Runner
	interfaces InterfaceLister
	log        *slog.Logger
	dir        string
	socketDir  string
	mu         sync.Mutex
}

// NewManager returns a manager writing the Kea files under dataDir/kea.
func NewManager(st *store.Store, sup supervisor.Supervisor, dataDir string, log *slog.Logger) *Manager {
	return &Manager{
		store:      st,
		sup:        sup,
		run:        execRunner,
		interfaces: hostInterfaces,
		log:        log,
		dir:        filepath.Join(dataDir, "kea"),
		socketDir:  DefaultSocketDir,
	}
}

// SetRunner, SetInterfaceLister and SetSocketDir replace the system
// interactions, for tests.
func (m *Manager) SetRunner(r Runner)                   { m.run = r }
func (m *Manager) SetInterfaceLister(l InterfaceLister) { m.interfaces = l }
func (m *Manager) SetSocketDir(dir string)              { m.socketDir = dir }

func serviceName(f Family) string {
	if f == V6 {
		return "kea-dhcp6"
	}
	return "kea-dhcp4"
}

func (m *Manager) confPath(f Family) string {
	return filepath.Join(m.dir, serviceName(f)+".conf")
}

func (m *Manager) client(f Family) *keaClient {
	return &keaClient{socket: filepath.Join(m.socketDir, socketName(f))}
}

// Init writes the Kea configurations and starts the enabled servers.
func (m *Manager) Init(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.prepareDirs(); err != nil {
		return err
	}
	settings, err := m.Settings(ctx)
	if err != nil {
		return err
	}
	if !m.sup.Available() {
		m.log.Warn("service supervision unavailable, Kea is not managed")
	}
	var errs []error
	for _, f := range []Family{V4, V6} {
		subnets, err := m.Subnets(ctx, f)
		if err != nil {
			return err
		}
		if err := m.activate(ctx, f, settings, subnets); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", f, err))
		}
	}
	return errors.Join(errs...)
}

// prepareDirs creates the data and socket directories, owned by the Kea user
// which the servers run as.
func (m *Manager) prepareDirs() error {
	uid, gid := -1, -1
	if u, err := user.Lookup(keaUser); err == nil {
		uid, _ = strconv.Atoi(u.Uid)
		gid, _ = strconv.Atoi(u.Gid)
	}
	for _, dir := range []string{m.dir, m.socketDir} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return err
		}
		if uid >= 0 {
			os.Chown(dir, uid, gid)
		}
	}
	return nil
}

// Settings returns the stored settings, or the defaults.
func (m *Manager) Settings(ctx context.Context) (Settings, error) {
	var s Settings
	err := m.store.GetSetting(ctx, settingsKey, &s)
	if errors.Is(err, store.ErrNotFound) {
		return DefaultSettings(), nil
	} else if err != nil {
		return Settings{}, err
	}
	s.Normalize()
	return s, nil
}

// UpdateSettings validates, stores and applies the global settings. It returns
// validation.Errors or a RejectedError when they are invalid, and ErrApply
// when they were stored but could not be applied.
func (m *Manager) UpdateSettings(ctx context.Context, s Settings) (Settings, error) {
	s.Normalize()
	if err := s.Validate(); err != nil {
		return Settings{}, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	subnets := map[Family][]Subnet{}
	var errs validation.Errors
	m.checkInterfacesExist(&errs, s.Interfaces)
	for _, f := range []Family{V4, V6} {
		list, err := m.Subnets(ctx, f)
		if err != nil {
			return Settings{}, err
		}
		subnets[f] = list
		for _, sub := range list {
			if sub.Interface != "" && !contains(s.Interfaces, sub.Interface) {
				errs.Add("interfaces", "%s is used by the subnet %s", sub.Interface, sub.Subnet)
			}
		}
	}
	if err := errs.Err(); err != nil {
		return Settings{}, err
	}
	for _, f := range []Family{V4, V6} {
		if err := m.test(ctx, f, s, subnets[f]); err != nil {
			return Settings{}, err
		}
	}
	if err := m.store.PutSetting(ctx, settingsKey, s); err != nil {
		return Settings{}, err
	}
	for _, f := range []Family{V4, V6} {
		if err := m.activate(ctx, f, s, subnets[f]); err != nil {
			m.log.Error("failed to apply DHCP settings", "family", f, "error", err)
			return s, fmt.Errorf("%w: %v", ErrApply, err)
		}
	}
	m.log.Info("DHCP settings applied", "dhcpv4", s.DHCPv4.Enabled, "dhcpv6", s.DHCPv6.Enabled)
	return s, nil
}

// checkInterfacesExist reports the interfaces missing on the host.
func (m *Manager) checkInterfacesExist(errs *validation.Errors, names []string) {
	list, err := m.interfaces()
	if err != nil {
		m.log.Warn("cannot list the network interfaces", "error", err)
		return
	}
	for i, name := range names {
		found := slices.ContainsFunc(list, func(n NetInterface) bool { return n.Name == name })
		if !found {
			errs.Add(fmt.Sprintf("interfaces[%d]", i), "interface %s not found on this host", name)
		}
	}
}

func subnetsKey(f Family) string {
	return "dhcp.subnets." + string(f)
}

// Subnets returns the subnets of a family, sorted by ID.
func (m *Manager) Subnets(ctx context.Context, f Family) ([]Subnet, error) {
	var list []Subnet
	err := m.store.GetSetting(ctx, subnetsKey(f), &list)
	if errors.Is(err, store.ErrNotFound) {
		return []Subnet{}, nil
	} else if err != nil {
		return nil, err
	}
	for i := range list {
		list[i].Normalize(f)
	}
	slices.SortFunc(list, func(a, b Subnet) int { return int(a.ID) - int(b.ID) })
	return list, nil
}

// Subnet returns a subnet of a family.
func (m *Manager) Subnet(ctx context.Context, f Family, id uint32) (Subnet, error) {
	list, err := m.Subnets(ctx, f)
	if err != nil {
		return Subnet{}, err
	}
	for _, s := range list {
		if s.ID == id {
			return s, nil
		}
	}
	return Subnet{}, ErrSubnetNotFound
}

// CreateSubnet adds a subnet and assigns its ID.
func (m *Manager) CreateSubnet(ctx context.Context, f Family, s Subnet) (Subnet, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	list, err := m.Subnets(ctx, f)
	if err != nil {
		return Subnet{}, err
	}
	if len(list) >= maxSubnets {
		return Subnet{}, validation.Errors{{Field: "subnet", Message: fmt.Sprintf("at most %d subnets are allowed", maxSubnets)}}
	}
	s.ID = 1
	for _, existing := range list {
		s.ID = max(s.ID, existing.ID+1)
	}
	if err := m.saveSubnet(ctx, f, list, s); err != nil {
		return Subnet{}, err
	}
	m.log.Info("DHCP subnet created", "family", f, "subnet", s.Subnet, "id", s.ID)
	return m.Subnet(ctx, f, s.ID)
}

// UpdateSubnet replaces a subnet. Its ID cannot change.
func (m *Manager) UpdateSubnet(ctx context.Context, f Family, id uint32, s Subnet) (Subnet, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	list, err := m.Subnets(ctx, f)
	if err != nil {
		return Subnet{}, err
	}
	i := slices.IndexFunc(list, func(e Subnet) bool { return e.ID == id })
	if i < 0 {
		return Subnet{}, ErrSubnetNotFound
	}
	s.ID = id
	if err := m.saveSubnet(ctx, f, slices.Delete(list, i, i+1), s); err != nil {
		return Subnet{}, err
	}
	return m.Subnet(ctx, f, id)
}

// DeleteSubnet removes a subnet. Its leases are kept by Kea until they expire.
func (m *Manager) DeleteSubnet(ctx context.Context, f Family, id uint32) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	list, err := m.Subnets(ctx, f)
	if err != nil {
		return err
	}
	i := slices.IndexFunc(list, func(e Subnet) bool { return e.ID == id })
	if i < 0 {
		return ErrSubnetNotFound
	}
	subnet := list[i].Subnet
	list = slices.Delete(list, i, i+1)
	settings, err := m.Settings(ctx)
	if err != nil {
		return err
	}
	if err := m.commit(ctx, f, settings, list); err != nil {
		return err
	}
	m.log.Info("DHCP subnet deleted", "family", f, "subnet", subnet, "id", id)
	return nil
}

// saveSubnet validates s, checks that it does not overlap the other subnets
// of the family, then stores and applies the new list.
func (m *Manager) saveSubnet(ctx context.Context, f Family, others []Subnet, s Subnet) error {
	settings, err := m.Settings(ctx)
	if err != nil {
		return err
	}
	s.Normalize(f)
	if err := s.Validate(f, settings.Interfaces); err != nil {
		return err
	}
	prefix := netip.MustParsePrefix(s.Subnet)
	for _, o := range others {
		if p, err := netip.ParsePrefix(o.Subnet); err == nil && p.Overlaps(prefix) {
			return validation.Errors{{Field: "subnet", Message: "overlaps the subnet " + o.Subnet}}
		}
	}
	list := append(slices.Clone(others), s)
	slices.SortFunc(list, func(a, b Subnet) int { return int(a.ID) - int(b.ID) })
	return m.commit(ctx, f, settings, list)
}

// commit tests, stores and applies the subnets of a family.
func (m *Manager) commit(ctx context.Context, f Family, settings Settings, list []Subnet) error {
	if err := m.test(ctx, f, settings, list); err != nil {
		return err
	}
	if err := m.store.PutSetting(ctx, subnetsKey(f), list); err != nil {
		return err
	}
	if err := m.activate(ctx, f, settings, list); err != nil {
		m.log.Error("failed to apply DHCP subnets", "family", f, "error", err)
		return fmt.Errorf("%w: %v", ErrApply, err)
	}
	return nil
}

func (m *Manager) render(f Family, settings Settings, subnets []Subnet) []byte {
	return render(renderInput{Family: f, Settings: settings, Subnets: subnets, DataDir: m.dir})
}

func (m *Manager) keaEnv() []string {
	return []string{"KEA_DHCP_DATA_DIR=" + m.dir, "KEA_CONTROL_SOCKET_DIR=" + m.socketDir}
}

// keaErrorRe extracts the reason of a refused configuration from the output
// of "kea-dhcp4 -t", without the position in the temporary file.
var keaErrorRe = regexp.MustCompile(`(?m)^Error encountered: (.*?)(?: \([^()]*:\d+:\d+\))?$`)

// test checks a configuration with Kea itself, which catches what the
// validation of the settings does not. It is skipped outside of the container.
func (m *Manager) test(ctx context.Context, f Family, settings Settings, subnets []Subnet) error {
	if !m.sup.Available() {
		return nil
	}
	path := m.confPath(f) + ".test"
	if err := os.WriteFile(path, m.render(f, settings, subnets), 0o600); err != nil {
		return err
	}
	defer os.Remove(path)
	out, err := m.run(ctx, m.keaEnv(), serviceName(f), "-t", path)
	if err == nil {
		return nil
	}
	msg := strings.TrimSpace(string(out))
	if match := keaErrorRe.FindStringSubmatch(msg); match != nil {
		msg = match[1]
	} else if lines := strings.Split(msg, "\n"); len(lines) > 0 && lines[len(lines)-1] != "" {
		msg = lines[len(lines)-1]
	} else {
		msg = err.Error()
	}
	return &RejectedError{Message: fmt.Sprintf("%s refused the configuration: %s", f, msg)}
}

// activate writes the configuration of a family and brings its server to the
// wanted state. A running server reloads its configuration without losing
// its leases.
func (m *Manager) activate(ctx context.Context, f Family, settings Settings, subnets []Subnet) error {
	enabled := settings.Family(f).Enabled
	service := serviceName(f)
	// Stop a disabled server before writing its configuration, so that it
	// does not start on it in the meantime.
	if !enabled && m.sup.Available() {
		if err := m.sup.Down(ctx, service); err != nil {
			return err
		}
	}
	changed, err := writeFile(m.confPath(f), m.render(f, settings, subnets), 0o640)
	if err != nil {
		return err
	}
	// The servers run as the Kea user and read the file through its group.
	if g, err := user.LookupGroup(keaUser); err == nil {
		if gid, err := strconv.Atoi(g.Gid); err == nil {
			os.Chown(m.confPath(f), 0, gid)
		}
	}
	if !m.sup.Available() || !enabled {
		return nil
	}
	up, _ := m.sup.IsUp(ctx, service)
	switch {
	case !changed && up:
		return nil
	case up:
		if err := m.client(f).command(ctx, "config-reload", nil, nil); err != nil {
			m.log.Warn("Kea reload failed, restarting it", "family", f, "error", err)
			return m.sup.Restart(ctx, service)
		}
		return nil
	default:
		return m.sup.Up(ctx, service)
	}
}

// running reports whether the server of a family is up.
func (m *Manager) running(ctx context.Context, f Family) bool {
	if !m.sup.Available() {
		return false
	}
	up, err := m.sup.IsUp(ctx, serviceName(f))
	return err == nil && up
}

// writeFile atomically replaces a file and reports whether its content changed.
func writeFile(path string, content []byte, mode os.FileMode) (bool, error) {
	if current, err := os.ReadFile(path); err == nil && string(current) == string(content) {
		return false, nil
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, content, mode); err != nil {
		return false, err
	}
	return true, os.Rename(tmp, path)
}

// NetInterface is a network interface of the host.
type NetInterface struct {
	Name       string   `json:"name"`
	HWAddress  string   `json:"hwAddress"`
	Up         bool     `json:"up"`
	Addresses  []string `json:"addresses"`
	IsLoopback bool     `json:"-"`
}

func hostInterfaces() ([]NetInterface, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	list := make([]NetInterface, 0, len(ifaces))
	for _, iface := range ifaces {
		n := NetInterface{
			Name:       iface.Name,
			HWAddress:  iface.HardwareAddr.String(),
			Up:         iface.Flags&net.FlagUp != 0,
			Addresses:  []string{},
			IsLoopback: iface.Flags&net.FlagLoopback != 0,
		}
		if addrs, err := iface.Addrs(); err == nil {
			for _, a := range addrs {
				n.Addresses = append(n.Addresses, a.String())
			}
		}
		list = append(list, n)
	}
	return list, nil
}

// Interfaces returns the network interfaces that DHCP can be served on.
func (m *Manager) Interfaces() ([]NetInterface, error) {
	list, err := m.interfaces()
	if err != nil {
		return nil, err
	}
	return slices.DeleteFunc(list, func(n NetInterface) bool { return n.IsLoopback }), nil
}
