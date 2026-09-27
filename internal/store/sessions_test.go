package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

// newTestStore opens an in-memory database with the schema applied, closed
// automatically at the end of the test.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	st, err := Open(":memory:")
	if err != nil {
		t.Fatalf("opening in-memory store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func TestCreateAndGetSession(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	sess, err := st.CreateSession(ctx, "admin")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if sess.ID == "" {
		t.Fatal("expected a non-empty session ID")
	}

	got, err := st.GetSession(ctx, sess.ID)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if got.Username != "admin" {
		t.Errorf("Username = %q, want admin", got.Username)
	}
}

func TestGetSession_UnknownID(t *testing.T) {
	st := newTestStore(t)

	_, err := st.GetSession(context.Background(), "does-not-exist")
	if !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("GetSession error = %v, want ErrSessionNotFound", err)
	}
}

func TestDeleteSession(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	sess, err := st.CreateSession(ctx, "admin")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if err := st.DeleteSession(ctx, sess.ID); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}

	if _, err := st.GetSession(ctx, sess.ID); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("GetSession after delete = %v, want ErrSessionNotFound", err)
	}
}

func TestGetSession_ExpiredIsTreatedAsNotFound(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	// Insert an already-expired session directly, bypassing CreateSession's
	// fixed TTL.
	id := "expired-session"
	_, err := st.db.ExecContext(ctx,
		`INSERT INTO sessions (id, username, created_at, expires_at) VALUES (?, ?, ?, ?)`,
		id, "admin", time.Now().Add(-2*time.Hour).Unix(), time.Now().Add(-time.Hour).Unix(),
	)
	if err != nil {
		t.Fatalf("inserting expired session: %v", err)
	}

	if _, err := st.GetSession(ctx, id); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("GetSession on expired session = %v, want ErrSessionNotFound", err)
	}
}

func TestDeleteExpiredSessions(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	live, err := st.CreateSession(ctx, "admin")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	expiredID := "expired-session"
	_, err = st.db.ExecContext(ctx,
		`INSERT INTO sessions (id, username, created_at, expires_at) VALUES (?, ?, ?, ?)`,
		expiredID, "admin", time.Now().Add(-2*time.Hour).Unix(), time.Now().Add(-time.Hour).Unix(),
	)
	if err != nil {
		t.Fatalf("inserting expired session: %v", err)
	}

	n, err := st.DeleteExpiredSessions(ctx)
	if err != nil {
		t.Fatalf("DeleteExpiredSessions: %v", err)
	}
	if n != 1 {
		t.Fatalf("deleted %d sessions, want 1", n)
	}

	if _, err := st.GetSession(ctx, live.ID); err != nil {
		t.Fatalf("live session was affected by cleanup: %v", err)
	}
}

func TestRunSessionCleanup_StopsWhenContextCanceled(t *testing.T) {
	st := newTestStore(t)
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		st.RunSessionCleanup(ctx, time.Millisecond)
		close(done)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("RunSessionCleanup did not return after context cancellation")
	}
}
