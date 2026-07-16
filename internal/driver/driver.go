// Package driver orchestrates the pipeline: read the Markdown file, extract
// the code, parse, check, generate Go, and hand the result to the Go
// toolchain in a temporary module. All diagnostics are reported against the
// original .md file.
package driver

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/justfedec/go-api-example/inkdown/internal/check"
	"github.com/justfedec/go-api-example/inkdown/internal/codegen"
	"github.com/justfedec/go-api-example/inkdown/internal/literate"
	"github.com/justfedec/go-api-example/inkdown/internal/parser"
	"github.com/justfedec/go-api-example/inkdown/internal/token"
)

// Options configures Build and Run.
type Options struct {
	Out    string // build: output binary path ("" derives it from the file name)
	EmitGo string // if set, also write the generated Go source to this path

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
// It returns the program's exit code.
func Run(file string, opts Options) (int, error) {
	bin, cleanup, err := compileToBinary(file, opts.EmitGo)
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
		return 1, fmt.Errorf("running %s: %w", file, err)
	}
	return 0, nil
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
