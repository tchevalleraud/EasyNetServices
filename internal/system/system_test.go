package system

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/tchevalleraud/EasyNetServices/internal/store"
	"github.com/tchevalleraud/EasyNetServices/internal/validation"
)

func TestTimezone(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	t.Setenv("TZ", "")
	defer location.Store(time.UTC)

	m := NewManager(st, slog.New(slog.NewTextHandler(io.Discard, nil)))
	s, err := m.Settings(ctx)
	if err != nil || s.Timezone != "UTC" {
		t.Fatalf("expected UTC by default, got %+v (%v)", s, err)
	}

	for _, bad := range []string{"", "Local", "Mars/Olympus"} {
		var errs validation.Errors
		if _, err := m.Update(ctx, Settings{Timezone: bad}); !errors.As(err, &errs) {
			t.Errorf("%q: expected validation error, got %v", bad, err)
		}
	}

	if _, err := m.Update(ctx, Settings{Timezone: "Europe/Paris"}); err != nil {
		t.Fatal(err)
	}
	m.now = func() time.Time { return time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC) }
	now := m.Now()
	if now.Timezone != "Europe/Paris" || now.UTCOffsetSeconds != 7200 || now.Abbreviation != "CEST" {
		t.Fatalf("unexpected time: %+v", now)
	}
	if now.FormattedLocal != "2026-07-01T12:00:00+02:00" || now.FormattedUTC != "2026-07-01T10:00:00Z" {
		t.Fatalf("unexpected formatting: %+v", now)
	}

	// A new manager must restore the stored time zone.
	location.Store(time.UTC)
	if err := NewManager(st, m.log).Init(ctx); err != nil {
		t.Fatal(err)
	}
	if Location().String() != "Europe/Paris" {
		t.Fatalf("expected stored time zone to be applied, got %s", Location())
	}
}
