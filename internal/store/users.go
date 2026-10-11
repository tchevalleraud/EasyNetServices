package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// User is an account allowed to access the management interface.
type User struct {
	ID                 int64
	Username           string
	Source             string
	PasswordHash       string
	MustChangePassword bool
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

const userColumns = `id, username, source, password_hash, must_change_password, created_at, updated_at`

func scanUser(row interface{ Scan(...any) error }) (*User, error) {
	var u User
	var created, updated int64
	if err := row.Scan(&u.ID, &u.Username, &u.Source, &u.PasswordHash, &u.MustChangePassword, &created, &updated); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	u.CreatedAt = time.Unix(created, 0)
	u.UpdatedAt = time.Unix(updated, 0)
	return &u, nil
}

// CountUsers returns the number of accounts.
func (s *Store) CountUsers(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

// GetUserByID returns the user with the given ID.
func (s *Store) GetUserByID(ctx context.Context, id int64) (*User, error) {
	return scanUser(s.db.QueryRowContext(ctx, `SELECT `+userColumns+` FROM users WHERE id = ?`, id))
}

// GetUserByUsername returns the user with the given username (case-insensitive).
func (s *Store) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	return scanUser(s.db.QueryRowContext(ctx, `SELECT `+userColumns+` FROM users WHERE username = ?`, username))
}

// CreateUser inserts a new user and returns it.
func (s *Store) CreateUser(ctx context.Context, u *User) (*User, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO users (username, source, password_hash, must_change_password) VALUES (?, ?, ?, ?)`,
		u.Username, u.Source, u.PasswordHash, u.MustChangePassword)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return s.GetUserByID(ctx, id)
}

// UpdatePassword replaces the password hash of a user.
func (s *Store) UpdatePassword(ctx context.Context, id int64, hash string, mustChange bool) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE users SET password_hash = ?, must_change_password = ?, updated_at = unixepoch() WHERE id = ?`,
		hash, mustChange, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
