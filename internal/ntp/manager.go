package ntp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/tchevalleraud/EasyNetServices/internal/store"
	"github.com/tchevalleraud/EasyNetServices/internal/supervisor"
)

const (
	settingsKey = "ntp"
	// serviceName is the s6 service running chronyd.
	serviceName = "chronyd"
)

var (
	// ErrApply is returned when the configuration was saved but could not be applied.
	ErrApply = errors.New("configuration saved but could not be applied")
	// ErrRejected is returned when chronyd refuses a configuration that passed
	// validation.
	ErrRejected = errors.New("chrony rejected the configuration")
)

// Runner executes a command and returns its combined output.
type Runner func(ctx context.Context, name string, args ...string) ([]byte, error)

func execRunner(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

// Manager stores the NTP configuration and applies it to chronyd.
type Manager struct {
	store    *store.Store
	sup      supervisor.Supervisor
	run      Runner
	log      *slog.Logger
	confFile string
	stateDir string
	mu       sync.Mutex
}

// NewManager returns a manager writing the chrony files under dataDir/chrony.
func NewManager(st *store.Store, sup supervisor.Supervisor, dataDir string, log *slog.Logger) *Manager {
	dir := filepath.Join(dataDir, "chrony")
	return &Manager{
		store:    st,
		sup:      sup,
		run:      execRunner,
		log:      log,
		confFile: filepath.Join(dir, "chrony.conf"),
		stateDir: filepath.Join(dir, "state"),
	}
}

// SetRunner replaces the function used to run chronyd and chronyc, for tests.
func (m *Manager) SetRunner(r Runner) {
	m.run = r
}

// Config returns the stored configuration, or the default one when none was saved.
func (m *Manager) Config(ctx context.Context) (Config, error) {
	var cfg Config
	err := m.store.GetSetting(ctx, settingsKey, &cfg)
	if errors.Is(err, store.ErrNotFound) {
		cfg = DefaultConfig()
	} else if err != nil {
		return Config{}, err
	}
	cfg.Normalize()
	return cfg, nil
}

// Init applies the stored configuration at startup.
func (m *Manager) Init(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cfg, err := m.Config(ctx)
	if err != nil {
		return err
	}
	return m.apply(ctx, cfg)
}

// Update validates, stores and applies a new configuration. It returns
// validation.Errors when the configuration is invalid, and ErrApply when it
// was stored but could not be applied.
func (m *Manager) Update(ctx context.Context, cfg Config) (Config, error) {
	cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.check(ctx, Render(cfg, m.stateDir)); err != nil {
		return Config{}, err
	}
	if err := m.store.PutSetting(ctx, settingsKey, cfg); err != nil {
		return Config{}, err
	}
	if err := m.apply(ctx, cfg); err != nil {
		m.log.Error("failed to apply NTP configuration", "error", err)
		return cfg, fmt.Errorf("%w: %v", ErrApply, err)
	}
	m.log.Info("NTP configuration applied", "enabled", cfg.Enabled, "sources", len(cfg.Sources))
	return cfg, nil
}

// check lets chronyd parse the rendered configuration, as a safety net on top
// of Validate. It is skipped when chronyd is not installed.
func (m *Manager) check(ctx context.Context, content string) error {
	tmp, err := os.CreateTemp("", "chrony-check-*.conf")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		return err
	}
	tmp.Close()

	out, err := m.run(ctx, "chronyd", "-p", "-f", tmp.Name())
	var execErr *exec.Error
	if errors.As(err, &execErr) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("%w: %s", ErrRejected, firstLine(out))
	}
	return nil
}

// apply writes the configuration file and brings chronyd to the wanted state.
func (m *Manager) apply(ctx context.Context, cfg Config) error {
	// Stop a disabled chronyd before writing its configuration, so that it
	// does not start on it in the meantime.
	if !cfg.Enabled && m.sup.Available() {
		if err := m.sup.Down(ctx, serviceName); err != nil {
			return err
		}
	}
	changed, err := m.writeConfig(Render(cfg, m.stateDir))
	if err != nil {
		return err
	}
	if !m.sup.Available() {
		m.log.Warn("service supervision unavailable, chronyd is not managed")
		return nil
	}
	if !cfg.Enabled {
		return nil
	}
	if changed {
		return m.sup.Restart(ctx, serviceName)
	}
	return m.sup.Up(ctx, serviceName)
}

// writeConfig atomically replaces the configuration file and reports whether
// its content changed.
func (m *Manager) writeConfig(content string) (bool, error) {
	if current, err := os.ReadFile(m.confFile); err == nil && bytes.Equal(current, []byte(content)) {
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(m.confFile), 0o755); err != nil {
		return false, err
	}
	tmp := m.confFile + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		return false, err
	}
	return true, os.Rename(tmp, m.confFile)
}

// Status returns the runtime state of chronyd.
func (m *Manager) Status(ctx context.Context) (Status, error) {
	status := Status{State: supervisor.StateUnavailable, Sources: []SourceStatus{}}

	cfg, err := m.Config(ctx)
	if err != nil {
		return status, err
	}
	if !m.sup.Available() {
		return status, nil
	}
	if !cfg.Enabled {
		status.State = supervisor.StateDisabled
		return status, nil
	}

	up, err := m.sup.IsUp(ctx, serviceName)
	if err != nil {
		return status, err
	}
	if !up {
		status.State = supervisor.StateStopped
		return status, nil
	}
	status.State = supervisor.StateRunning

	// chronyd may still be starting; missing data is reported as unsynchronized.
	if out, err := m.run(ctx, "chronyc", "-n", "-c", "tracking"); err == nil {
		if t, err := parseTracking(string(out)); err == nil {
			status.Tracking = t
			status.Synchronized = t.synchronized()
		} else {
			m.log.Debug("cannot parse chronyc tracking", "error", err)
		}
	}
	if out, err := m.run(ctx, "chronyc", "-n", "-c", "sources"); err == nil {
		if sources, err := parseSources(string(out)); err == nil {
			status.Sources = sources
		} else {
			m.log.Debug("cannot parse chronyc sources", "error", err)
		}
	}
	return status, nil
}

// firstLine returns the first line of a chronyd error, without its timestamp.
func firstLine(b []byte) string {
	s := strings.TrimSpace(string(b))
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	if i := strings.Index(s, "Fatal error : "); i >= 0 {
		s = s[i+len("Fatal error : "):]
	}
	return s
}
