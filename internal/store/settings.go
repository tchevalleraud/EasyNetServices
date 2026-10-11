package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

// GetSetting decodes the JSON document stored under key into v.
// It returns ErrNotFound when the key does not exist.
func (s *Store) GetSetting(ctx context.Context, key string, v any) error {
	var raw string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, key).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(raw), v)
}

// PutSetting stores v as a JSON document under key, replacing any previous value.
func (s *Store) PutSetting(ctx context.Context, key string, v any) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO settings (key, value) VALUES (?, ?)
		 ON CONFLICT (key) DO UPDATE SET value = excluded.value, updated_at = unixepoch()`,
		key, string(raw))
	return err
}
