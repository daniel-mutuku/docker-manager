package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ErrSessionNotFound is returned when a session id has no matching row, or
// the row has expired.
var ErrSessionNotFound = errors.New("session not found")

// SessionTTL is how long a session remains valid after creation.
const SessionTTL = 24 * time.Hour

// Session represents an authenticated browser session.
type Session struct {
	ID        string
	Username  string
	ExpiresAt time.Time
}

// CreateSession inserts a new session for username and returns it.
func (s *Store) CreateSession(ctx context.Context, username string) (*Session, error) {
	sess := &Session{
		ID:        uuid.NewString(),
		Username:  username,
		ExpiresAt: time.Now().Add(SessionTTL),
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO sessions (id, username, created_at, expires_at) VALUES (?, ?, ?, ?)`,
		sess.ID, sess.Username, time.Now().Unix(), sess.ExpiresAt.Unix(),
	)
	if err != nil {
		return nil, fmt.Errorf("creating session: %w", err)
	}
	return sess, nil
}

// GetSession looks up a non-expired session by id.
func (s *Store) GetSession(ctx context.Context, id string) (*Session, error) {
	var sess Session
	var expiresAt int64
	err := s.db.QueryRowContext(ctx,
		`SELECT id, username, expires_at FROM sessions WHERE id = ?`, id,
	).Scan(&sess.ID, &sess.Username, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrSessionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("querying session: %w", err)
	}
	sess.ExpiresAt = time.Unix(expiresAt, 0)
	if sess.ExpiresAt.Before(time.Now()) {
		_ = s.DeleteSession(ctx, id)
		return nil, ErrSessionNotFound
	}
	return &sess, nil
}

// DeleteSession removes a session (used on logout).
func (s *Store) DeleteSession(ctx context.Context, id string) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE id = ?`, id); err != nil {
		return fmt.Errorf("deleting session: %w", err)
	}
	return nil
}
