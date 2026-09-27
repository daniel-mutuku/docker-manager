package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrStackNotFound is returned when a stack name has no matching row.
var ErrStackNotFound = errors.New("stack not found")

// ErrStackExists is returned when creating a stack whose name is already taken.
var ErrStackExists = errors.New("stack already exists")

// Stack is a saved Compose stack definition.
type Stack struct {
	ID             int64
	Name           string
	ComposeContent string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// CreateStack saves a new stack definition. It returns ErrStackExists if the
// name is already taken.
func (s *Store) CreateStack(ctx context.Context, name, composeContent string) (*Stack, error) {
	now := time.Now()
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO stacks (name, compose_content, created_at, updated_at) VALUES (?, ?, ?, ?)`,
		name, composeContent, now.Unix(), now.Unix(),
	)
	if err != nil {
		if isUniqueConstraintErr(err) {
			return nil, ErrStackExists
		}
		return nil, fmt.Errorf("creating stack: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("reading new stack id: %w", err)
	}
	return &Stack{ID: id, Name: name, ComposeContent: composeContent, CreatedAt: now, UpdatedAt: now}, nil
}

// GetStack looks up a stack by name.
func (s *Store) GetStack(ctx context.Context, name string) (*Stack, error) {
	var st Stack
	var createdAt, updatedAt int64
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, compose_content, created_at, updated_at FROM stacks WHERE name = ?`, name,
	).Scan(&st.ID, &st.Name, &st.ComposeContent, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrStackNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("querying stack: %w", err)
	}
	st.CreatedAt = time.Unix(createdAt, 0)
	st.UpdatedAt = time.Unix(updatedAt, 0)
	return &st, nil
}

// ListStacks returns every saved stack, most recently updated first.
func (s *Store) ListStacks(ctx context.Context) ([]Stack, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, compose_content, created_at, updated_at FROM stacks ORDER BY updated_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("listing stacks: %w", err)
	}
	defer rows.Close()

	var stacks []Stack
	for rows.Next() {
		var st Stack
		var createdAt, updatedAt int64
		if err := rows.Scan(&st.ID, &st.Name, &st.ComposeContent, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scanning stack row: %w", err)
		}
		st.CreatedAt = time.Unix(createdAt, 0)
		st.UpdatedAt = time.Unix(updatedAt, 0)
		stacks = append(stacks, st)
	}
	return stacks, rows.Err()
}

// UpdateStackContent replaces a stack's saved Compose content, e.g. before a
// redeploy. It returns ErrStackNotFound if the name doesn't exist.
func (s *Store) UpdateStackContent(ctx context.Context, name, composeContent string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE stacks SET compose_content = ?, updated_at = ? WHERE name = ?`,
		composeContent, time.Now().Unix(), name,
	)
	if err != nil {
		return fmt.Errorf("updating stack: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking update result: %w", err)
	}
	if n == 0 {
		return ErrStackNotFound
	}
	return nil
}

// DeleteStack removes a stack's saved definition. It does not tear down any
// containers the stack may have deployed; callers should do that first.
func (s *Store) DeleteStack(ctx context.Context, name string) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM stacks WHERE name = ?`, name); err != nil {
		return fmt.Errorf("deleting stack: %w", err)
	}
	return nil
}

func isUniqueConstraintErr(err error) bool {
	return strings.Contains(err.Error(), "UNIQUE constraint")
}
