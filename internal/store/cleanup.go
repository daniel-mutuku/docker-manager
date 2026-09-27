package store

import (
	"context"
	"fmt"
	"log"
	"time"
)

// DeleteExpiredSessions removes every session row whose expiry has passed
// and reports how many were deleted.
func (s *Store) DeleteExpiredSessions(ctx context.Context) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at < ?`, time.Now().Unix())
	if err != nil {
		return 0, fmt.Errorf("deleting expired sessions: %w", err)
	}
	return res.RowsAffected()
}

// RunSessionCleanup periodically deletes expired sessions until ctx is
// canceled. Run it in its own goroutine; it returns once ctx is done, so
// callers can rely on it not leaking past shutdown.
func (s *Store) RunSessionCleanup(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if n, err := s.DeleteExpiredSessions(ctx); err != nil {
				log.Printf("session cleanup: %v", err)
			} else if n > 0 {
				log.Printf("session cleanup: removed %d expired session(s)", n)
			}
		}
	}
}
