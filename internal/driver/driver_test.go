package driver

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestMain points the binary cache at a throwaway directory so the suite
// never reads or writes the developer's real cache.
func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "inkdown-test-cache-")
	if err != nil {
		panic(err)
	}
	os.Setenv("INKDOWN_CACHE_DIR", tmp)
	code := m.Run()
	os.RemoveAll(tmp)
	os.Exit(code)
}

func TestBuildCache(t *testing.T) {
	cacheRoot := t.TempDir()
	t.Setenv("INKDOWN_CACHE_DIR", cacheRoot)

	md := filepath.Join(t.TempDir(), "cacheme.md")
	program := "# c\n\n```inkdown\nprint(6 * 7)\n```\n"
	if err := os.WriteFile(md, []byte(program), 0o644); err != nil {
		t.Fatal(err)
	}

	countBinaries := func() int {
		n := 0
		entries, _ := os.ReadDir(cacheRoot)
		for _, e := range entries {
			if _, err := os.Stat(filepath.Join(cacheRoot, e.Name(), "program")); err == nil {
				n++
			}
		}
		return n
	}

	run := func() string {
		var out bytes.Buffer
		code, err := Run(md, Options{Stdout: &out})
		if err != nil || code != 0 {
			t.Fatalf("Run: %v code %d", err, code)
		}
		return out.String()
	}

	// First run compiles and populates the cache.
	if got := run(); got != "42\n" {
		t.Fatalf("first run = %q", got)
	}
	if n := countBinaries(); n != 1 {
		t.Fatalf("after first run, cached binaries = %d, want 1", n)
	}

	// Second run of the same program hits the cache: still one entry, and
	// the cached binary is not rebuilt (its build time is unchanged).
	bin := filepath.Join(cacheRoot, mustOneEntry(t, cacheRoot), "program")
	before, _ := os.Stat(bin)
	buildTime := before.ModTime()
	time.Sleep(10 * time.Millisecond)
	if got := run(); got != "42\n" {
		t.Fatalf("second run = %q", got)
	}
	if n := countBinaries(); n != 1 {
		t.Fatalf("after cache hit, cached binaries = %d, want 1", n)
	}
	// A hit touches mtime forward (used-marker) but must not be older than
	// the original build — i.e. it was not recompiled into a new slot.
	after, _ := os.Stat(bin)
	if after.ModTime().Before(buildTime) {
		t.Errorf("cached binary went backwards in time; it was rebuilt")
	}

	// Editing the program changes the generated Go, so the key changes and a
	// second entry appears.
	if err := os.WriteFile(md, []byte("# c\n\n```inkdown\nprint(7 * 7)\n```\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := run(); got != "49\n" {
		t.Fatalf("edited run = %q", got)
	}
	if n := countBinaries(); n != 2 {
		t.Fatalf("after edit, cached binaries = %d, want 2", n)
	}

	// --no-cache still runs but adds no new entry.
	var out bytes.Buffer
	if _, err := Run(md, Options{Stdout: &out, NoCache: true}); err != nil {
		t.Fatal(err)
	}
	if n := countBinaries(); n != 2 {
		t.Errorf("--no-cache changed the cache count to %d, want 2", n)
	}
}

func mustOneEntry(t *testing.T, dir string) string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() {
			return e.Name()
		}
	}
	t.Fatal("no cache entry directory found")
	return ""
}

// TestExamplesGolden compiles and runs every example program, comparing its
// stdout against the sibling .out file.
func TestExamplesGolden(t *testing.T) {
	examples, err := filepath.Glob(filepath.Join("..", "..", "examples", "*.md"))
	if err != nil || len(examples) == 0 {
		t.Fatalf("no examples found: %v", err)
	}
	for _, md := range examples {
		md := md
		t.Run(filepath.Base(md), func(t *testing.T) {
			t.Parallel()
			want, err := os.ReadFile(strings.TrimSuffix(md, ".md") + ".out")
			if err != nil {
				// Examples that need the network or a terminal ship without
				// a golden file, but they must still compile.
				if cerr := Check(md); cerr != nil {
					t.Fatalf("Check: %v", cerr)
				}
				t.Skipf("no golden .out file; type-checked only")
			}
			var stdout, stderr bytes.Buffer
			code, err := Run(md, Options{Stdout: &stdout, Stderr: &stderr})
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			if code != 0 {
				t.Fatalf("exit code = %d, stderr:\n%s", code, stderr.String())
			}
			if stdout.String() != string(want) {
				t.Errorf("output mismatch\n got:\n%s\nwant:\n%s", stdout.String(), want)
			}
		})
	}
}

func TestDiagnosticsPointAtMarkdownLines(t *testing.T) {
	err := Check(filepath.Join("testdata", "typeerror.md"))
	if err == nil {
		t.Fatal("expected a type error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "typeerror.md:13:13") {
		t.Errorf("error not mapped to the Markdown line: %q", msg)
	}
	if !strings.Contains(msg, "mismatched operand types (string and int)") {
		t.Errorf("unexpected message: %q", msg)
	}
}

func TestNoCodeBlocks(t *testing.T) {
	err := Check(filepath.Join("testdata", "nocode.md"))
	if err == nil || !strings.Contains(err.Error(), "no ```inkdown code blocks") {
		t.Errorf("err = %v, want a no-code-blocks diagnostic", err)
	}
}

// TestRuntimePanicMapsToMarkdown checks the //line directives: a panic's
// stack trace must reference the .md file and line, and the process must
// exit nonzero.
func TestRuntimePanicMapsToMarkdown(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, err := Run(filepath.Join("testdata", "panics.md"), Options{Stdout: &stdout, Stderr: &stderr})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if code == 0 {
		t.Error("expected a nonzero exit code")
	}
	out := stderr.String()
	if !strings.Contains(out, "index out of range") {
		t.Errorf("stderr missing panic message:\n%s", out)
	}
	if !strings.Contains(out, "panics.md:8") {
		t.Errorf("stderr does not point at panics.md:8:\n%s", out)
	}
}

func TestBuildProducesBinary(t *testing.T) {
	out := filepath.Join(t.TempDir(), "hello")
	if err := Build(filepath.Join("..", "..", "examples", "hello.md"), Options{Out: out}); err != nil {
		t.Fatalf("Build: %v", err)
	}
	got, err := exec.Command(out).Output()
	if err != nil {
		t.Fatalf("running built binary: %v", err)
	}
	if want := "Hello, world!\n"; string(got) != want {
		t.Errorf("binary output = %q, want %q", got, want)
	}
}

func TestEmitGo(t *testing.T) {
	emit := filepath.Join(t.TempDir(), "hello.go")
	var stdout bytes.Buffer
	if _, err := Run(filepath.Join("..", "..", "examples", "hello.md"), Options{EmitGo: emit, Stdout: &stdout}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	src, err := os.ReadFile(emit)
	if err != nil {
		t.Fatalf("emitted Go not written: %v", err)
	}
	for _, want := range []string{"package main", "//line hello.md:", "fmt.Println"} {
		if !strings.Contains(string(src), want) {
			t.Errorf("emitted Go missing %q", want)
		}
	}
}

func TestDefaultOutName(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"examples/hello.md", "hello"},
		{"prog", "prog.bin"},
		{".md", ".md.bin"},
	} {
		if got := defaultOutName(tc.in); got != tc.want {
			t.Errorf("defaultOutName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
