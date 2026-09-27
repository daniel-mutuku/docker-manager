package store

import (
	"context"
	"errors"
	"testing"
)

func TestCreateAndGetStack(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	created, err := st.CreateStack(ctx, "my-app", "services:\n  web:\n    image: nginx\n")
	if err != nil {
		t.Fatalf("CreateStack: %v", err)
	}
	if created.ID == 0 {
		t.Error("expected a non-zero stack ID")
	}

	got, err := st.GetStack(ctx, "my-app")
	if err != nil {
		t.Fatalf("GetStack: %v", err)
	}
	if got.ComposeContent != "services:\n  web:\n    image: nginx\n" {
		t.Errorf("ComposeContent = %q, want the content passed to CreateStack", got.ComposeContent)
	}
}

func TestCreateStack_DuplicateNameRejected(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	if _, err := st.CreateStack(ctx, "dup", "a"); err != nil {
		t.Fatalf("first CreateStack: %v", err)
	}
	_, err := st.CreateStack(ctx, "dup", "b")
	if !errors.Is(err, ErrStackExists) {
		t.Fatalf("second CreateStack error = %v, want ErrStackExists", err)
	}
}

func TestGetStack_UnknownName(t *testing.T) {
	st := newTestStore(t)

	_, err := st.GetStack(context.Background(), "does-not-exist")
	if !errors.Is(err, ErrStackNotFound) {
		t.Fatalf("GetStack error = %v, want ErrStackNotFound", err)
	}
}

func TestListStacks(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	if _, err := st.CreateStack(ctx, "a", "content-a"); err != nil {
		t.Fatalf("CreateStack a: %v", err)
	}
	if _, err := st.CreateStack(ctx, "b", "content-b"); err != nil {
		t.Fatalf("CreateStack b: %v", err)
	}

	stacks, err := st.ListStacks(ctx)
	if err != nil {
		t.Fatalf("ListStacks: %v", err)
	}
	if len(stacks) != 2 {
		t.Fatalf("got %d stacks, want 2", len(stacks))
	}
}

func TestUpdateStackContent(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	if _, err := st.CreateStack(ctx, "my-app", "old"); err != nil {
		t.Fatalf("CreateStack: %v", err)
	}
	if err := st.UpdateStackContent(ctx, "my-app", "new"); err != nil {
		t.Fatalf("UpdateStackContent: %v", err)
	}

	got, err := st.GetStack(ctx, "my-app")
	if err != nil {
		t.Fatalf("GetStack: %v", err)
	}
	if got.ComposeContent != "new" {
		t.Errorf("ComposeContent = %q, want new", got.ComposeContent)
	}
}

func TestUpdateStackContent_UnknownName(t *testing.T) {
	st := newTestStore(t)

	err := st.UpdateStackContent(context.Background(), "does-not-exist", "content")
	if !errors.Is(err, ErrStackNotFound) {
		t.Fatalf("UpdateStackContent error = %v, want ErrStackNotFound", err)
	}
}

func TestDeleteStack(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	if _, err := st.CreateStack(ctx, "my-app", "content"); err != nil {
		t.Fatalf("CreateStack: %v", err)
	}
	if err := st.DeleteStack(ctx, "my-app"); err != nil {
		t.Fatalf("DeleteStack: %v", err)
	}
	if _, err := st.GetStack(ctx, "my-app"); !errors.Is(err, ErrStackNotFound) {
		t.Fatalf("GetStack after delete = %v, want ErrStackNotFound", err)
	}
}
