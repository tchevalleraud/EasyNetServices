package auth

import (
	"context"
	"errors"

	"github.com/tchevalleraud/EasyNetServices/internal/store"
)

// PasswordProvider authenticates a user from a username and a password.
//
// The local database is the first implementation; an LDAP provider will implement
// the same interface. Redirect-based protocols such as OIDC will use a dedicated
// flow but end up creating a session through Service in the same way.
type PasswordProvider interface {
	// Name identifies the provider and matches the "source" column of users.
	Name() string
	// Authenticate returns the user matching the credentials, or ErrInvalidCredentials.
	Authenticate(ctx context.Context, username, password string) (*store.User, error)
}

// LocalProvider authenticates users stored in the local database.
type LocalProvider struct {
	store *store.Store
	// dummyHash is verified when the user does not exist, so that response time
	// does not reveal which usernames are valid.
	dummyHash string
}

// NewLocalProvider returns a provider backed by the local database.
func NewLocalProvider(s *store.Store) (*LocalProvider, error) {
	dummy, err := HashPassword("dummy-password")
	if err != nil {
		return nil, err
	}
	return &LocalProvider{store: s, dummyHash: dummy}, nil
}

// Name implements PasswordProvider.
func (p *LocalProvider) Name() string { return "local" }

// Authenticate implements PasswordProvider.
func (p *LocalProvider) Authenticate(ctx context.Context, username, password string) (*store.User, error) {
	user, err := p.store.GetUserByUsername(ctx, username)
	if errors.Is(err, store.ErrNotFound) || (err == nil && user.Source != p.Name()) {
		VerifyPassword(password, p.dummyHash)
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, err
	}
	ok, err := VerifyPassword(password, user.PasswordHash)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrInvalidCredentials
	}
	return user, nil
}
