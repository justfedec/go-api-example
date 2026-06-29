package main

import (
	"bufio"
	"os"
	"strings"
)

// loadEnvFile loads KEY=VALUE pairs from a dotenv-style file into the process
// environment, without overriding variables already set. A missing file is a
// no-op (e.g. local dev, where DATABASE_URL is exported directly).
//
// Why this exists: on the Propie platform, attached Databases and other env
// vars are written to /app/.env as raw `KEY=VALUE` lines — they are NOT placed
// in the process environment. Node/Next.js apps read .env automatically; a plain
// Go process does not, so we load it here before main reads DATABASE_URL.
//
// Values are taken literally (split on the first '=' only), so connection
// strings containing '=' (e.g. `?sslmode=disable`) are preserved. We do not
// shell-source the file precisely because connection strings can contain
// characters a shell would mangle.
func loadEnvFile(path string) {
	f, err := os.Open(path)
	if err != nil {
		return // no file — rely on the real environment
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, val)
		}
	}
}
