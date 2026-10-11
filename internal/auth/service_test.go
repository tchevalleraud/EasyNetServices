package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/tchevalleraud/EasyNetServices/internal/store"
)

func newTestService(t *testing.T) (*Service, *store.Store) {
	t.Helper()
	st, err := store.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	local, err := NewLocalProvider(st)
	if err != nil {
		t.Fatal(err)
	}
	return NewService(st, time.Hour, local), st
}

func TestPasswordHashRoundTrip(t *testing.T) {
	hash, err := HashPassword("s3cret-pass")
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := VerifyPassword("s3cret-pass", hash); err != nil || !ok {
		t.Fatalf("expected password to match, got ok=%v err=%v", ok, err)
	}
	if ok, _ := VerifyPassword("wrong", hash); ok {
		t.Fatal("expected wrong password to be rejected")
	}
	if _, err := VerifyPassword("x", "not-a-hash"); err == nil {
		t.Fatal("expected malformed hash error")
	}
}

func TestValidateNewPassword(t *testing.T) {
	cases := map[string]bool{
		"short":          false,
		"password":       false,
		"PASSWORD":       false,
		"current-pass":   false,
		"a-good-pass-42": true,
	}
	for pw, valid := range cases {
		err := ValidateNewPassword(pw, "current-pass")
		if valid && err != nil {
			t.Errorf("%q: unexpected error %v", pw, err)
		}
		if !valid && !errors.Is(err, ErrWeakPassword) {
			t.Errorf("%q: expected ErrWeakPassword, got %v", pw, err)
		}
	}
}

func TestDefaultAdminLifecycle(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)

	created, err := svc.EnsureDefaultAdmin(ctx)
	if err != nil || !created {
		t.Fatalf("expected admin to be created, got created=%v err=%v", created, err)
	}
	if created, _ := svc.EnsureDefaultAdmin(ctx); created {
		t.Fatal("admin must only be created once")
	}

	if _, _, err := svc.Login(ctx, "admin", "wrong"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected invalid credentials, got %v", err)
	}
	if _, _, err := svc.Login(ctx, "nobody", "password"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected invalid credentials, got %v", err)
	}

	token, user, err := svc.Login(ctx, "ADMIN", DefaultAdminPassword)
	if err != nil {
		t.Fatal(err)
	}
	if !user.MustChangePassword {
		t.Fatal("default admin must be forced to change its password")
	}

	// A second session that must be closed by the password change.
	otherToken, _, err := svc.Login(ctx, "admin", DefaultAdminPassword)
	if err != nil {
		t.Fatal(err)
	}

	if err := svc.ChangePassword(ctx, user, token, "wrong", "new-password-1"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected invalid credentials, got %v", err)
	}
	if err := svc.ChangePassword(ctx, user, token, DefaultAdminPassword, "new-password-1"); err != nil {
		t.Fatal(err)
	}

	user, err = svc.Authenticate(ctx, token)
	if err != nil {
		t.Fatalf("current session must survive the password change: %v", err)
	}
	if user.MustChangePassword {
		t.Fatal("password change flag must be cleared")
	}
	if _, err := svc.Authenticate(ctx, otherToken); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("other sessions must be closed, got %v", err)
	}

	if err := svc.Logout(ctx, token); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Authenticate(ctx, token); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("expected invalid session after logout, got %v", err)
	}
}

func TestResetPassword(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	if _, err := svc.EnsureDefaultAdmin(ctx); err != nil {
		t.Fatal(err)
	}

	temp, err := svc.ResetPassword(ctx, "admin")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.Login(ctx, "admin", DefaultAdminPassword); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatal("old password must no longer work")
	}
	_, user, err := svc.Login(ctx, "admin", temp)
	if err != nil {
		t.Fatal(err)
	}
	if !user.MustChangePassword {
		t.Fatal("a reset password must be changed at next login")
	}
}

func TestSessionExpiration(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	if _, err := svc.EnsureDefaultAdmin(ctx); err != nil {
		t.Fatal(err)
	}

	svc.now = func() time.Time { return time.Now().Add(-2 * time.Hour) }
	token, _, err := svc.Login(ctx, "admin", DefaultAdminPassword)
	if err != nil {
		t.Fatal(err)
	}
	svc.now = time.Now
	if _, err := svc.Authenticate(ctx, token); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("expected expired session, got %v", err)
	}
}
