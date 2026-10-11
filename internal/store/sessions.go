package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// Session is an authenticated browser session.
type Session struct {
	ID        string
	UserID    int64
	CreatedAt time.Time
	ExpiresAt time.Time
}

// CreateSession stores a new session.
func (s *Store) CreateSession(ctx context.Context, sess *Session) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO sessions (id, user_id, created_at, expires_at) VALUES (?, ?, ?, ?)`,
		sess.ID, sess.UserID, sess.CreatedAt.Unix(), sess.ExpiresAt.Unix())
	return err
}

// GetSession returns a session that has not expired yet.
func (s *Store) GetSession(ctx context.Context, id string) (*Session, error) {
	var sess Session
	var created, expires int64
	err := s.db.QueryRowContext(ctx,
		`SELECT id, user_id, created_at, expires_at FROM sessions WHERE id = ? AND expires_at > unixepoch()`, id).
		Scan(&sess.ID, &sess.UserID, &created, &expires)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	sess.CreatedAt = time.Unix(created, 0)
	sess.ExpiresAt = time.Unix(expires, 0)
	return &sess, nil
}

// ExtendSession pushes back the expiration of a session.
func (s *Store) ExtendSession(ctx context.Context, id string, expiresAt time.Time) error {
	_, err := s.db.ExecContext(ctx, `UPDATE sessions SET expires_at = ? WHERE id = ?`, expiresAt.Unix(), id)
	return err
}

// DeleteSession removes a session.
func (s *Store) DeleteSession(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE id = ?`, id)
	return err
}

// DeleteUserSessions removes all sessions of a user except the one given in keep (may be empty).
func (s *Store) DeleteUserSessions(ctx context.Context, userID int64, keep string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ? AND id != ?`, userID, keep)
	return err
}

// DeleteExpiredSessions purges expired sessions.
func (s *Store) DeleteExpiredSessions(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at <= unixepoch()`)
	return err
}
