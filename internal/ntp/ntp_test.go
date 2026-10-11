package ntp

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/tchevalleraud/EasyNetServices/internal/store"
	"github.com/tchevalleraud/EasyNetServices/internal/validation"
)

func TestValidate(t *testing.T) {
	if err := func() error { c := DefaultConfig(); return c.Validate() }(); err != nil {
		t.Fatalf("default config must be valid: %v", err)
	}

	cfg := Config{
		Enabled: true,
		Sources: []Source{
			{Type: "bogus", Address: "pool.ntp.org"},
			{Type: SourceServer, Address: "evil\nallow all"},
			{Type: SourceServer, Address: "pool.ntp.org"},
		},
		Access:       []AccessRule{{Action: "maybe", Network: "10.0.0.1"}},
		LocalStratum: 16,
	}
	err := cfg.Validate()
	var errs validation.Errors
	if !errors.As(err, &errs) {
		t.Fatalf("expected validation errors, got %v", err)
	}
	want := []string{
		"sources[0].type", "sources[1].address", "sources[2].address",
		"access[0].action", "access[0].network", "localStratum",
	}
	got := map[string]bool{}
	for _, e := range errs {
		got[e.Field] = true
	}
	for _, f := range want {
		if !got[f] {
			t.Errorf("missing error on %s (got %v)", f, errs)
		}
	}

	empty := Config{Enabled: true}
	if empty.Validate() == nil {
		t.Error("an enabled server without source nor local stratum must be rejected")
	}
	empty.LocalStratum = 10
	if err := empty.Validate(); err != nil {
		t.Errorf("local stratum without sources must be accepted: %v", err)
	}
}

func TestNormalizeMasksNetworks(t *testing.T) {
	cfg := Config{Access: []AccessRule{{Action: AccessAllow, Network: "192.168.1.42/24"}}}
	cfg.Normalize()
	if cfg.Access[0].Network != "192.168.1.0/24" || cfg.Sources == nil {
		t.Fatalf("unexpected normalized config: %+v", cfg)
	}
}

func TestRender(t *testing.T) {
	cfg := Config{
		Enabled: true,
		Sources: []Source{
			{Type: SourcePool, Address: "pool.ntp.org", IBurst: true},
			{Type: SourceServer, Address: "time.cloudflare.com", IBurst: true, Prefer: true, NTS: true},
		},
		Access: []AccessRule{
			{Action: AccessAllow, Network: "192.168.0.0/16"},
			{Action: AccessDeny, Network: "192.168.66.0/24"},
		},
		LocalStratum: 10,
	}
	out := Render(cfg, "/data/chrony/state")
	for _, line := range []string{
		"pool pool.ntp.org iburst\n",
		"server time.cloudflare.com iburst prefer nts\n",
		"allow 192.168.0.0/16\ndeny 192.168.66.0/24\n",
		"local stratum 10\n",
		"driftfile /data/chrony/state/chrony.drift\n",
		"cmdport 0\n",
	} {
		if !strings.Contains(out, line) {
			t.Errorf("missing %q in:\n%s", line, out)
		}
	}
}

func TestParseTracking(t *testing.T) {
	out := "A29FC801,162.159.200.1,4,1760220000.123456789,-0.000012345,0.000001,0.0001,-12.345,0.001,0.02,0.012,0.0005,64.5,Normal\n"
	tr, err := parseTracking(out)
	if err != nil {
		t.Fatal(err)
	}
	if tr.Reference != "162.159.200.1" || tr.Stratum != 4 || tr.OffsetSeconds != -0.000012345 || !tr.synchronized() {
		t.Fatalf("unexpected tracking: %+v", tr)
	}

	unsync := "00000000,,0,0.000000000,0.000000000,0.000000000,0.000000000,0.000,0.000,0.000,1.000000000,1.000000000,0.0,Not synchronised\n"
	tr, err = parseTracking(unsync)
	if err != nil {
		t.Fatal(err)
	}
	if tr.synchronized() {
		t.Fatal("expected unsynchronized clock")
	}

	if _, err := parseTracking("garbage"); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestParseSources(t *testing.T) {
	out := "^,*,162.159.200.1,3,6,377,34,0.000123,0.000110,0.012\n^,?,192.0.2.1,0,10,0,4294967295,0.0,0.0,0.0\n"
	sources, err := parseSources(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) != 2 {
		t.Fatalf("expected 2 sources, got %d", len(sources))
	}
	s := sources[0]
	if s.Mode != "server" || s.State != "selected" || s.PollSeconds != 64 || s.Reach != "377" || s.LastRxSeconds != 34 {
		t.Fatalf("unexpected source: %+v", s)
	}
	if sources[1].State != "unreachable" || sources[1].LastRxSeconds != -1 {
		t.Fatalf("unexpected source: %+v", sources[1])
	}
}

type fakeSupervisor struct {
	available bool
	up        bool
	calls     []string
}

func (f *fakeSupervisor) Available() bool { return f.available }
func (f *fakeSupervisor) Up(_ context.Context, name string) error {
	f.calls = append(f.calls, "up "+name)
	f.up = true
	return nil
}
func (f *fakeSupervisor) Down(_ context.Context, name string) error {
	f.calls = append(f.calls, "down "+name)
	f.up = false
	return nil
}
func (f *fakeSupervisor) Restart(_ context.Context, name string) error {
	f.calls = append(f.calls, "restart "+name)
	f.up = true
	return nil
}
func (f *fakeSupervisor) IsUp(context.Context, string) (bool, error) { return f.up, nil }

func newTestManager(t *testing.T) (*Manager, *fakeSupervisor) {
	t.Helper()
	dir := t.TempDir()
	st, err := store.Open(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	sup := &fakeSupervisor{available: true}
	m := NewManager(st, sup, dir, slog.New(slog.NewTextHandler(io.Discard, nil)))
	m.run = func(_ context.Context, name string, args ...string) ([]byte, error) {
		switch strings.Join(append([]string{name}, args...), " ") {
		case "chronyc -n -c tracking":
			return []byte("A29FC801,162.159.200.1,4,1760220000.1,-0.00001,0,0,0,0,0,0.01,0.001,64,Normal\n"), nil
		case "chronyc -n -c sources":
			return []byte("^,*,162.159.200.1,3,6,377,34,0.000123,0.000110,0.012\n"), nil
		}
		return nil, nil // chronyd -p accepts every configuration
	}
	return m, sup
}

func TestManagerLifecycle(t *testing.T) {
	ctx := context.Background()
	m, sup := newTestManager(t)

	if err := m.Init(ctx); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(m.confFile)
	if err != nil || !strings.Contains(string(content), "pool pool.ntp.org iburst") {
		t.Fatalf("default configuration must be written, got %q (%v)", content, err)
	}
	if got := strings.Join(sup.calls, ","); got != "restart chronyd" {
		t.Fatalf("unexpected supervisor calls: %s", got)
	}

	// Applying the same configuration again must not restart chronyd.
	sup.calls = nil
	if err := m.Init(ctx); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(sup.calls, ","); got != "up chronyd" {
		t.Fatalf("unexpected supervisor calls: %s", got)
	}

	status, err := m.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != StateRunning || !status.Synchronized || len(status.Sources) != 1 {
		t.Fatalf("unexpected status: %+v", status)
	}

	cfg := DefaultConfig()
	cfg.Enabled = false
	if _, err := m.Update(ctx, cfg); err != nil {
		t.Fatal(err)
	}
	if status, _ := m.Status(ctx); status.State != StateDisabled {
		t.Fatalf("expected disabled state, got %s", status.State)
	}
	stored, _ := m.Config(ctx)
	if stored.Enabled {
		t.Fatal("configuration must be persisted")
	}

	cfg.Sources[0].Address = "bad address"
	var errs validation.Errors
	if _, err := m.Update(ctx, cfg); !errors.As(err, &errs) {
		t.Fatalf("expected validation errors, got %v", err)
	}
}

func TestManagerRejectsConfigRefusedByChrony(t *testing.T) {
	m, _ := newTestManager(t)
	m.run = func(context.Context, string, ...string) ([]byte, error) {
		return []byte("2026-10-11T22:00:36Z Fatal error : Could not parse server directive at line 1\n"), errors.New("exit status 1")
	}
	_, err := m.Update(context.Background(), DefaultConfig())
	if !errors.Is(err, ErrRejected) || !strings.Contains(err.Error(), "Could not parse server directive") {
		t.Fatalf("expected chrony error, got %v", err)
	}
}
