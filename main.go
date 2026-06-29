package main

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

//go:embed index.html
var static embed.FS

func main() {
	addr := flag.String("addr", "0.0.0.0:8080", "listen address")
	flag.Parse()

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, nil)))

	// The platform injects env vars (e.g. an attached Database's DATABASE_URL)
	// into /app/.env, not the process environment. Load it so os.Getenv sees
	// them. No-op locally where the file is absent and you export DATABASE_URL.
	loadEnvFile("/app/.env")

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		slog.Error("DATABASE_URL is required — attach a Database to this VM (the platform injects it as DATABASE_URL)")
		os.Exit(1)
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		slog.Error("open database failed", "err", err)
		os.Exit(1)
	}
	defer db.Close()
	db.SetMaxOpenConns(10)
	db.SetConnMaxIdleTime(5 * time.Minute)

	store := NewPostgresStore(db)

	// The DB may still be warming on a fresh VM — retry the ping before failing.
	if err := waitForDB(store, 30*time.Second); err != nil {
		slog.Error("database unreachable", "err", err)
		os.Exit(1)
	}

	migCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	err = store.Migrate(migCtx)
	cancel()
	if err != nil {
		slog.Error("migrate failed", "err", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", handleIndex)
	mux.HandleFunc("GET /todos", listTodos(store))
	mux.HandleFunc("POST /todos", createTodo(store))
	mux.HandleFunc("PATCH /todos/{id}", toggleTodo(store))
	mux.HandleFunc("DELETE /todos/{id}", deleteTodo(store))
	mux.HandleFunc("GET /health", handleHealth(store))

	srv := &http.Server{
		Addr:         *addr,
		Handler:      logRequests(mux),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		slog.Info("server started", "addr", *addr, "store", "postgres")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("listen failed", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down...")
	shutdownCtx, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown error", "err", err)
	}
	slog.Info("server stopped")
}

// waitForDB pings until success or the deadline elapses.
func waitForDB(store *PostgresStore, within time.Duration) error {
	deadline := time.Now().Add(within)
	var lastErr error
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		err := store.Ping(ctx)
		cancel()
		if err == nil {
			return nil
		}
		lastErr = err
		slog.Warn("waiting for database...", "err", err)
		time.Sleep(time.Second)
	}
	return lastErr
}

func handleHealth(s *PostgresStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := s.Ping(ctx); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "down", "store": "postgres"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "store": "postgres"})
	}
}

// --- Middleware ---

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		slog.Info("request", "method", r.Method, "path", r.URL.Path, "duration", time.Since(start))
	})
}

// --- Handlers ---

func handleIndex(w http.ResponseWriter, _ *http.Request) {
	data, _ := static.ReadFile("index.html")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}

func listTodos(s Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		todos, err := s.All(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not list todos")
			return
		}
		writeJSON(w, http.StatusOK, todos)
	}
}

func createTodo(s Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			Title string `json:"title"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input.Title == "" {
			writeError(w, http.StatusBadRequest, "title is required")
			return
		}
		todo, err := s.Add(r.Context(), input.Title)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not create todo")
			return
		}
		writeJSON(w, http.StatusCreated, todo)
	}
}

func toggleTodo(s Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid id")
			return
		}
		todo, err := s.Toggle(r.Context(), id)
		if errors.Is(err, ErrNotFound) {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not toggle todo")
			return
		}
		writeJSON(w, http.StatusOK, todo)
	}
}

func deleteTodo(s Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid id")
			return
		}
		if err := s.Delete(r.Context(), id); errors.Is(err, ErrNotFound) {
			writeError(w, http.StatusNotFound, "not found")
			return
		} else if err != nil {
			writeError(w, http.StatusInternalServerError, "could not delete todo")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// --- Helpers ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
