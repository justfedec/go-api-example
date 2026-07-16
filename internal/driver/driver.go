// Package driver orchestrates the pipeline: read the Markdown file, extract
// the code, parse, check, generate Go, and hand the result to the Go
// toolchain in a temporary module. All diagnostics are reported against the
// original .md file.
package driver

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/justfedec/inkdown/internal/check"
	"github.com/justfedec/inkdown/internal/codegen"
	"github.com/justfedec/inkdown/internal/literate"
	"github.com/justfedec/inkdown/internal/parser"
	"github.com/justfedec/inkdown/internal/token"
)

// Options configures Build and Run.
type Options struct {
	Out     string // build: output binary path ("" derives it from the file name)
	EmitGo  string // if set, also write the generated Go source to this path
	NoCache bool   // Run: skip the compiled-binary cache

	// Standard streams for the executed program (Run only); nil means the
	// process defaults.
	Stdout io.Writer
	Stderr io.Writer
	Stdin  io.Reader
}

// Check parses and type-checks file without generating code.
func Check(file string) error {
	_, err := CompileToGo(file)
	return err
}

// CompileToGo runs the front and middle end and returns the generated Go
// source. Diagnostics are formatted as file:line:col with Markdown lines.
func CompileToGo(file string) ([]byte, error) {
	src, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	lit, err := literate.Extract(string(src))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	prog, err := parser.Parse(lit.Code)
	if err != nil {
		return nil, mapErr(file, lit, err)
	}
	if err := check.Check(prog); err != nil {
		return nil, mapErr(file, lit, err)
	}
	return codegen.Generate(prog, filepath.Base(file), lit.MdLine), nil
}

// mapErr renders a stage diagnostic against the Markdown document.
func mapErr(file string, lit *literate.Program, err error) error {
	var te *token.Error
	if errors.As(err, &te) {
		return fmt.Errorf("%s:%d:%d: %s", file, lit.MdLine(te.Pos.Line), te.Pos.Col, te.Msg)
	}
	return fmt.Errorf("%s: %w", file, err)
}

// Build compiles file to a native binary.
func Build(file string, opts Options) error {
	out := opts.Out
	if out == "" {
		out = defaultOutName(file)
	}
	bin, cleanup, err := compileToBinary(file, opts.EmitGo)
	if err != nil {
		return err
	}
	defer cleanup()
	return moveFile(bin, out)
}

// Run compiles file and executes it, wiring the standard streams from opts.
// It returns the program's exit code. Unless caching is disabled, an
// unchanged program (same generated Go, same Go toolchain) skips the ~2s
// `go build` and executes a cached native binary directly.
func Run(file string, opts Options) (int, error) {
	bin, cleanup, cacheEntry, err := binaryFor(file, opts)
	if err != nil {
		return 1, err
	}
	defer cleanup()

	cmd := exec.Command(bin)
	cmd.Stdout = orDefault[io.Writer](opts.Stdout, os.Stdout)
	cmd.Stderr = orDefault[io.Writer](opts.Stderr, os.Stderr)
	cmd.Stdin = orDefault[io.Reader](opts.Stdin, os.Stdin)
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return exit.ExitCode(), nil
		}
		// The binary would not start (corrupt cache entry, or one built for a
		// different target). If it came from the cache, evict it and recompile
		// once with caching off.
		if cacheEntry != "" {
			os.RemoveAll(cacheEntry)
			return Run(file, Options{EmitGo: opts.EmitGo, NoCache: true,
				Stdout: opts.Stdout, Stderr: opts.Stderr, Stdin: opts.Stdin})
		}
		return 1, fmt.Errorf("running %s: %w", file, err)
	}
	return 0, nil
}

// binaryFor returns a runnable binary for file, using the cache when enabled.
// A cache hit returns the cached path (and its entry dir, so a failed exec can
// evict it) with a no-op cleanup; a miss compiles, stores the binary in the
// cache, and returns it. When --emit-go is set (or the cache directory is
// unusable), it falls back to a throwaway build with an empty entry.
func binaryFor(file string, opts Options) (bin string, cleanup func(), cacheEntry string, err error) {
	goSrc, err := CompileToGo(file)
	if err != nil {
		return "", nil, "", err
	}
	if opts.EmitGo != "" {
		if werr := os.WriteFile(opts.EmitGo, goSrc, 0o644); werr != nil {
			return "", nil, "", werr
		}
	}

	dir := cacheDir()
	if opts.NoCache || opts.EmitGo != "" || dir == "" {
		bin, cleanup, err = compileToBinaryFrom(goSrc, file)
		return bin, cleanup, "", err
	}

	entry := filepath.Join(dir, cacheKey(goSrc))
	cachedBin := filepath.Join(entry, "program")
	if _, statErr := os.Stat(cachedBin); statErr == nil {
		now := time.Now()
		os.Chtimes(cachedBin, now, now) // mark as recently used for pruning
		return cachedBin, func() {}, entry, nil
	}

	bin, cleanup, err = compileToBinaryFrom(goSrc, file)
	if err != nil {
		return "", nil, "", err
	}
	// Populate the cache atomically: copy into the entry dir, then rename
	// within the same filesystem. Any failure just leaves the cache unfilled.
	if err := storeInCache(entry, cachedBin, bin); err == nil {
		cleanup()
		pruneCache(dir)
		return cachedBin, func() {}, entry, nil
	}
	return bin, cleanup, "", nil
}

func orDefault[T comparable](v, def T) T {
	var zero T
	if v == zero {
		return def
	}
	return v
}

// compileToBinary generates Go code and builds it in a temporary module.
// The caller must invoke cleanup, which removes the temporary directory.
func compileToBinary(file, emitGo string) (bin string, cleanup func(), err error) {
	goSrc, err := CompileToGo(file)
	if err != nil {
		return "", nil, err
	}
	if emitGo != "" {
		if err := os.WriteFile(emitGo, goSrc, 0o644); err != nil {
			return "", nil, err
		}
	}
	return compileToBinaryFrom(goSrc, file)
}

// compileToBinaryFrom builds already-generated Go in a temporary module.
func compileToBinaryFrom(goSrc []byte, file string) (bin string, cleanup func(), err error) {
	goTool, err := exec.LookPath("go")
	if err != nil {
		return "", nil, errors.New("the Go toolchain is required but 'go' was not found in PATH")
	}

	dir, err := os.MkdirTemp("", "inkdown-")
	if err != nil {
		return "", nil, err
	}
	cleanup = func() { os.RemoveAll(dir) }
	fail := func(err error) (string, func(), error) {
		cleanup()
		return "", nil, err
	}

	if err := os.WriteFile(filepath.Join(dir, "main.go"), goSrc, 0o644); err != nil {
		return fail(err)
	}
	mod := "module inkdown_program\n\ngo 1.22\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(mod), 0o644); err != nil {
		return fail(err)
	}

	bin = filepath.Join(dir, "program")
	cmd := exec.Command(goTool, "build", "-o", bin, ".")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fail(fmt.Errorf("internal error: the generated Go failed to compile (this is an inkdown bug, run with --emit-go to inspect):\n%s", out))
	}
	return bin, cleanup, nil
}

func defaultOutName(file string) string {
	base := filepath.Base(file)
	if trimmed := strings.TrimSuffix(base, ".md"); trimmed != base && trimmed != "" {
		return trimmed
	}
	return base + ".bin"
}

// moveFile renames src to dst, falling back to a copy across filesystems.
func moveFile(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	return copyFile(src, dst)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// -------------------------------------------------------------- binary cache

// cacheKey derives a cache directory name from the generated Go source and
// the Go toolchain. Keying on the *generated Go* — not the .md bytes — means
// any change to the compiler or stdlib self-invalidates the cache, and the
// source name baked into the //line directives keeps same-content, different
// -name programs apart.
func cacheKey(goSrc []byte) string {
	h := sha256.New()
	h.Write(goSrc)
	h.Write([]byte{0})
	h.Write([]byte(goToolchainID()))
	return hex.EncodeToString(h.Sum(nil))[:16]
}

var (
	toolchainOnce sync.Once
	toolchainID   string
)

// goToolchainID returns the identity of the `go` in PATH — its version line
// plus the effective target platform (GOOS/GOARCH honor env vars and
// `go env -w`, which the version line does not reflect) — cached once per
// process.
func goToolchainID() string {
	toolchainOnce.Do(func() {
		goTool, err := exec.LookPath("go")
		if err != nil {
			toolchainID = "no-go"
			return
		}
		ver, err := exec.Command(goTool, "version").Output()
		if err != nil {
			toolchainID = goTool
			return
		}
		target, _ := exec.Command(goTool, "env", "GOOS", "GOARCH").Output()
		toolchainID = strings.TrimSpace(string(ver)) + "|" + strings.TrimSpace(string(target))
	})
	return toolchainID
}

// cacheDir resolves the binary cache root, honoring INKDOWN_CACHE_DIR and
// falling back to the user cache directory. Returns "" when no writable
// location exists, which disables caching (never an error).
func cacheDir() string {
	base := os.Getenv("INKDOWN_CACHE_DIR")
	if base == "" {
		ucd, err := os.UserCacheDir()
		if err != nil {
			return ""
		}
		base = filepath.Join(ucd, "inkdown")
	}
	if err := os.MkdirAll(base, 0o755); err != nil {
		return ""
	}
	return base
}

// storeInCache moves the freshly built binary into its cache entry via a
// same-directory temp file and an atomic rename.
func storeInCache(entry, cachedBin, builtBin string) error {
	if err := os.MkdirAll(entry, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(entry, "program.*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	tmp.Close()
	if err := copyFile(builtBin, tmpName); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, 0o755); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, cachedBin); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
}

// pruneCache best-effort removes entries whose binary hasn't been used in
// over 30 days. Errors are ignored — pruning is housekeeping, not correctness.
func pruneCache(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-30 * 24 * time.Hour)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		bin := filepath.Join(dir, e.Name(), "program")
		info, err := os.Stat(bin)
		if err != nil || info.ModTime().After(cutoff) {
			continue
		}
		os.RemoveAll(filepath.Join(dir, e.Name()))
	}
}
