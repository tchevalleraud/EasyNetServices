// Package auth handles user authentication and sessions.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/tchevalleraud/EasyNetServices/internal/store"
)

// Default account created on first start.
const (
	DefaultAdminUsername = "admin"
	DefaultAdminPassword = "password"
)

var (
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrInvalidSession     = errors.New("invalid or expired session")
	ErrWeakPassword       = errors.New("password does not meet the requirements")
	ErrNotLocalUser       = errors.New("password is managed by an external provider")
)

// Service is the entry point for authentication operations.
type Service struct {
	store      *store.Store
	providers  []PasswordProvider
	sessionTTL time.Duration
	now        func() time.Time
}

// NewService returns an authentication service trying each provider in order.
func NewService(s *store.Store, sessionTTL time.Duration, providers ...PasswordProvider) *Service {
	return &Service{store: s, providers: providers, sessionTTL: sessionTTL, now: time.Now}
}

// EnsureDefaultAdmin creates the default administrator when no user exists yet.
// It reports whether the account was created.
func (s *Service) EnsureDefaultAdmin(ctx context.Context) (bool, error) {
	n, err := s.store.CountUsers(ctx)
	if err != nil || n > 0 {
		return false, err
	}
	hash, err := HashPassword(DefaultAdminPassword)
	if err != nil {
		return false, err
	}
	_, err = s.store.CreateUser(ctx, &store.User{
		Username:           DefaultAdminUsername,
		Source:             "local",
		PasswordHash:       hash,
		MustChangePassword: true,
	})
	return err == nil, err
}

// Login checks the credentials and opens a session. It returns the session token
// to hand to the client.
func (s *Service) Login(ctx context.Context, username, password string) (string, *store.User, error) {
	for _, p := range s.providers {
		user, err := p.Authenticate(ctx, username, password)
		if errors.Is(err, ErrInvalidCredentials) {
			continue
		}
		if err != nil {
			return "", nil, fmt.Errorf("provider %s: %w", p.Name(), err)
		}
		token, err := s.createSession(ctx, user.ID)
		if err != nil {
			return "", nil, err
		}
		return token, user, nil
	}
	return "", nil, ErrInvalidCredentials
}

// Authenticate resolves a session token into its user, sliding the expiration
// when more than half of the lifetime has elapsed.
func (s *Service) Authenticate(ctx context.Context, token string) (*store.User, error) {
	id := hashToken(token)
	sess, err := s.store.GetSession(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		return nil, ErrInvalidSession
	}
	if err != nil {
		return nil, err
	}
	user, err := s.store.GetUserByID(ctx, sess.UserID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, ErrInvalidSession
	}
	if err != nil {
		return nil, err
	}
	if now := s.now(); sess.ExpiresAt.Sub(now) < s.sessionTTL/2 {
		if err := s.store.ExtendSession(ctx, id, now.Add(s.sessionTTL)); err != nil {
			return nil, err
		}
	}
	return user, nil
}

// Logout closes the session identified by token.
func (s *Service) Logout(ctx context.Context, token string) error {
	return s.store.DeleteSession(ctx, hashToken(token))
}

// ChangePassword updates the password of a local user after checking the current
// one, and closes all the other sessions of that user.
func (s *Service) ChangePassword(ctx context.Context, user *store.User, currentToken, current, next string) error {
	if user.Source != "local" {
		return ErrNotLocalUser
	}
	ok, err := VerifyPassword(current, user.PasswordHash)
	if err != nil {
		return err
	}
	if !ok {
		return ErrInvalidCredentials
	}
	if err := ValidateNewPassword(next, current); err != nil {
		return err
	}
	hash, err := HashPassword(next)
	if err != nil {
		return err
	}
	if err := s.store.UpdatePassword(ctx, user.ID, hash, false); err != nil {
		return err
	}
	return s.store.DeleteUserSessions(ctx, user.ID, hashToken(currentToken))
}

// ResetPassword sets a random temporary password on a local user, forces a change
// at next login and closes all of its sessions. It returns the temporary password.
func (s *Service) ResetPassword(ctx context.Context, username string) (string, error) {
	user, err := s.store.GetUserByUsername(ctx, username)
	if err != nil {
		return "", err
	}
	if user.Source != "local" {
		return "", ErrNotLocalUser
	}
	temp, err := RandomPassword()
	if err != nil {
		return "", err
	}
	hash, err := HashPassword(temp)
	if err != nil {
		return "", err
	}
	if err := s.store.UpdatePassword(ctx, user.ID, hash, true); err != nil {
		return "", err
	}
	return temp, s.store.DeleteUserSessions(ctx, user.ID, "")
}

// PurgeExpiredSessions removes expired sessions from the database.
func (s *Service) PurgeExpiredSessions(ctx context.Context) error {
	return s.store.DeleteExpiredSessions(ctx)
}

func (s *Service) createSession(ctx context.Context, userID int64) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(b)
	now := s.now()
	err := s.store.CreateSession(ctx, &store.Session{
		ID:        hashToken(token),
		UserID:    userID,
		CreatedAt: now,
		ExpiresAt: now.Add(s.sessionTTL),
	})
	return token, err
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
