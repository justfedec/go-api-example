package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadEnvFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	// Includes: a comment, a blank line, a value containing '=' (connection
	// string), and a key that is already set in the environment.
	content := "# platform-injected env\n\n" +
		"DATABASE_URL=postgres://u:p@h:5432/db?sslmode=disable\n" +
		"PRESET=fromfile\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	t.Setenv("PRESET", "preset-wins") // already set → must NOT be overridden
	// Register DATABASE_URL for restoration on cleanup (loadEnvFile sets it
	// via os.Setenv, which would otherwise leak into later tests), then unset
	// it so loadEnvFile takes the value from the file.
	t.Setenv("DATABASE_URL", "")
	os.Unsetenv("DATABASE_URL")

	loadEnvFile(path)

	if got := os.Getenv("DATABASE_URL"); got != "postgres://u:p@h:5432/db?sslmode=disable" {
		t.Fatalf("DATABASE_URL = %q, want the full connection string with sslmode", got)
	}
	if got := os.Getenv("PRESET"); got != "preset-wins" {
		t.Fatalf("PRESET = %q, want it left untouched (no override)", got)
	}
}

func TestLoadEnvFileMissingIsNoop(t *testing.T) {
	// Must not panic or set anything when the file is absent.
	loadEnvFile(filepath.Join(t.TempDir(), "does-not-exist.env"))
}
