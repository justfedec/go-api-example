package main

import (
	"context"
	"database/sql"
	"errors"
)

// PostgresStore is a Store backed by a Postgres table via database/sql + the
// pgx stdlib driver.
type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(db *sql.DB) *PostgresStore { return &PostgresStore{db: db} }

// Migrate creates the todos table if it doesn't exist. Idempotent — safe on
// every boot.
func (s *PostgresStore) Migrate(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS todos (
	id         SERIAL PRIMARY KEY,
	title      TEXT NOT NULL,
	completed  BOOLEAN NOT NULL DEFAULT false,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
)`)
	return err
}

// Ping verifies the database connection.
func (s *PostgresStore) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }

func (s *PostgresStore) All(ctx context.Context) ([]Todo, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, title, completed, created_at FROM todos ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	todos := []Todo{}
	for rows.Next() {
		var t Todo
		if err := rows.Scan(&t.ID, &t.Title, &t.Completed, &t.CreatedAt); err != nil {
			return nil, err
		}
		todos = append(todos, t)
	}
	return todos, rows.Err()
}

func (s *PostgresStore) Add(ctx context.Context, title string) (Todo, error) {
	var t Todo
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO todos (title) VALUES ($1)
		 RETURNING id, title, completed, created_at`, title).
		Scan(&t.ID, &t.Title, &t.Completed, &t.CreatedAt)
	return t, err
}

func (s *PostgresStore) Toggle(ctx context.Context, id int) (Todo, error) {
	var t Todo
	err := s.db.QueryRowContext(ctx,
		`UPDATE todos SET completed = NOT completed WHERE id = $1
		 RETURNING id, title, completed, created_at`, id).
		Scan(&t.ID, &t.Title, &t.Completed, &t.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Todo{}, ErrNotFound
	}
	return t, err
}

func (s *PostgresStore) Delete(ctx context.Context, id int) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM todos WHERE id = $1`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
