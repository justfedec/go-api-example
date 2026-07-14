package driver

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

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
				t.Fatalf("every example needs a golden .out file: %v", err)
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
