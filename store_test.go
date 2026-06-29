package main

import (
	"context"
	"testing"
)

// runStoreSuite exercises the Store contract against any implementation.
func runStoreSuite(t *testing.T, s Store) {
	t.Helper()
	ctx := context.Background()

	all, err := s.All(ctx)
	if err != nil {
		t.Fatalf("All: %v", err)
	}
	if len(all) != 0 {
		t.Fatalf("expected empty store, got %d", len(all))
	}

	a, err := s.Add(ctx, "first")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if a.ID == 0 || a.Title != "first" || a.Completed {
		t.Fatalf("unexpected todo: %+v", a)
	}

	tg, err := s.Toggle(ctx, a.ID)
	if err != nil {
		t.Fatalf("Toggle: %v", err)
	}
	if !tg.Completed {
		t.Fatalf("expected completed after toggle")
	}

	if _, err := s.Toggle(ctx, 99999); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound on missing toggle, got %v", err)
	}

	if err := s.Delete(ctx, a.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := s.Delete(ctx, a.ID); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound on second delete, got %v", err)
	}

	all, err = s.All(ctx)
	if err != nil {
		t.Fatalf("All after delete: %v", err)
	}
	if len(all) != 0 {
		t.Fatalf("expected empty after delete, got %d", len(all))
	}
}

func TestMemoryStore(t *testing.T) {
	runStoreSuite(t, NewMemoryStore())
}
