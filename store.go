package main

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ErrNotFound is returned by a Store when a todo id doesn't exist.
var ErrNotFound = errors.New("todo not found")

// Todo is the domain record. JSON shape is the app's public contract.
type Todo struct {
	ID        int       `json:"id"`
	Title     string    `json:"title"`
	Completed bool      `json:"completed"`
	CreatedAt time.Time `json:"createdAt"`
}

// Store is the persistence boundary for todos. The HTTP handlers depend only on
// this interface, never on a concrete backend.
type Store interface {
	All(ctx context.Context) ([]Todo, error)
	Add(ctx context.Context, title string) (Todo, error)
	Toggle(ctx context.Context, id int) (Todo, error)
	Delete(ctx context.Context, id int) error
}

// MemoryStore is an in-process Store backed by a slice. Retained as a reference
// implementation and for tests; production uses PostgresStore.
type MemoryStore struct {
	mu     sync.RWMutex
	todos  []Todo
	nextID int
}

func NewMemoryStore() *MemoryStore { return &MemoryStore{nextID: 1} }

func (s *MemoryStore) All(_ context.Context) ([]Todo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Todo, len(s.todos))
	copy(out, s.todos)
	return out, nil
}

func (s *MemoryStore) Add(_ context.Context, title string) (Todo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t := Todo{ID: s.nextID, Title: title, CreatedAt: time.Now().UTC()}
	s.nextID++
	s.todos = append(s.todos, t)
	return t, nil
}

func (s *MemoryStore) Toggle(_ context.Context, id int) (Todo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.todos {
		if s.todos[i].ID == id {
			s.todos[i].Completed = !s.todos[i].Completed
			return s.todos[i], nil
		}
	}
	return Todo{}, ErrNotFound
}

func (s *MemoryStore) Delete(_ context.Context, id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.todos {
		if s.todos[i].ID == id {
			s.todos = append(s.todos[:i], s.todos[i+1:]...)
			return nil
		}
	}
	return ErrNotFound
}
