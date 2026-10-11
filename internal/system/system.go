// Package system manages the settings of the appliance itself, such as its
// time zone.
package system

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tchevalleraud/EasyNetServices/internal/store"
	"github.com/tchevalleraud/EasyNetServices/internal/validation"
)

const settingsKey = "system"

// location is the configured time zone. time.Local is not modified because it
// must not change while other goroutines use it.
var location atomic.Pointer[time.Location]

func init() {
	location.Store(time.UTC)
}

// Location returns the configured time zone.
func Location() *time.Location {
	return location.Load()
}

// Settings are the system settings.
type Settings struct {
	// Timezone is an IANA time zone name such as "Europe/Paris". It is used to
	// display dates and in the logs; NTP itself always works in UTC.
	Timezone string `json:"timezone"`
}

// Time is the current time of the server.
type Time struct {
	// UnixMillis is the current time in milliseconds since the Unix epoch.
	UnixMillis       int64  `json:"unixMillis"`
	Timezone         string `json:"timezone"`
	Abbreviation     string `json:"abbreviation"`
	UTCOffsetSeconds int    `json:"utcOffsetSeconds"`
	FormattedLocal   string `json:"formattedLocal"`
	FormattedUTC     string `json:"formattedUtc"`
}

// Manager stores the system settings and applies them to the process.
type Manager struct {
	store *store.Store
	log   *slog.Logger
	mu    sync.Mutex
	now   func() time.Time
}

// NewManager returns a system settings manager.
func NewManager(st *store.Store, log *slog.Logger) *Manager {
	return &Manager{store: st, log: log, now: time.Now}
}

// defaultSettings uses the TZ environment variable, the usual way to set the
// time zone of a container, and falls back to UTC.
func defaultSettings() Settings {
	tz := os.Getenv("TZ")
	if _, err := loadLocation(tz); err != nil {
		tz = "UTC"
	}
	return Settings{Timezone: tz}
}

// Settings returns the stored settings, or the defaults when none were saved.
func (m *Manager) Settings(ctx context.Context) (Settings, error) {
	var s Settings
	err := m.store.GetSetting(ctx, settingsKey, &s)
	if errors.Is(err, store.ErrNotFound) {
		return defaultSettings(), nil
	}
	return s, err
}

// Init applies the stored settings at startup.
func (m *Manager) Init(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, err := m.Settings(ctx)
	if err != nil {
		return err
	}
	return m.apply(s)
}

// Update validates, stores and applies new settings. It returns
// validation.Errors when they are invalid.
func (m *Manager) Update(ctx context.Context, s Settings) (Settings, error) {
	var errs validation.Errors
	if _, err := loadLocation(s.Timezone); err != nil {
		errs.Add("timezone", "unknown time zone")
	}
	if err := errs.Err(); err != nil {
		return Settings{}, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.store.PutSetting(ctx, settingsKey, s); err != nil {
		return Settings{}, err
	}
	if err := m.apply(s); err != nil {
		return Settings{}, err
	}
	m.log.Info("time zone changed", "timezone", s.Timezone)
	return s, nil
}

// Now returns the current time of the server in the configured time zone.
func (m *Manager) Now() Time {
	loc := Location()
	now := m.now().In(loc)
	abbr, offset := now.Zone()
	return Time{
		UnixMillis:       now.UnixMilli(),
		Timezone:         loc.String(),
		Abbreviation:     abbr,
		UTCOffsetSeconds: offset,
		FormattedLocal:   now.Format(time.RFC3339),
		FormattedUTC:     now.UTC().Format(time.RFC3339),
	}
}

// apply sets the time zone used by the logs and by Now.
func (m *Manager) apply(s Settings) error {
	loc, err := loadLocation(s.Timezone)
	if err != nil {
		return fmt.Errorf("time zone %q: %w", s.Timezone, err)
	}
	location.Store(loc)
	return nil
}

// loadLocation loads an IANA time zone, rejecting the ambiguous "Local" and
// empty names accepted by time.LoadLocation.
func loadLocation(name string) (*time.Location, error) {
	if name == "" || name == "Local" {
		return nil, errors.New("unknown time zone")
	}
	return time.LoadLocation(name)
}
