// Command inkdown is the compiler and runner for the Inkdown language:
// literate programs written in Markdown, compiled to native binaries through
// Go. See SPEC.md for the language and README.md for usage.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/justfedec/inkdown/internal/driver"
)

const usage = `inkdown — a Markdown programming language compiled with Go

usage:
  inkdown run [--emit-go file.go] program.md     compile and execute
  inkdown build [-o binary] [--emit-go file.go] program.md
                                                 compile to a native binary
  inkdown check program.md                       parse and type-check only
`

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, usage)
		return 2
	}

	cmd, rest := args[0], args[1:]
	switch cmd {
	case "run":
		fs := flag.NewFlagSet("run", flag.ExitOnError)
		emitGo := fs.String("emit-go", "", "also write the generated Go source to this path")
		noCache := fs.Bool("no-cache", false, "always recompile instead of using the cached binary")
		fs.Parse(rest)
		file, ok := oneFile(fs)
		if !ok {
			return 2
		}
		code, err := driver.Run(file, driver.Options{EmitGo: *emitGo, NoCache: *noCache})
		if err != nil {
			fmt.Fprintln(os.Stderr, "inkdown:", err)
			return 1
		}
		return code

	case "build":
		fs := flag.NewFlagSet("build", flag.ExitOnError)
		out := fs.String("o", "", "output binary path (default: program name without .md)")
		emitGo := fs.String("emit-go", "", "also write the generated Go source to this path")
		fs.Parse(rest)
		file, ok := oneFile(fs)
		if !ok {
			return 2
		}
		if err := driver.Build(file, driver.Options{Out: *out, EmitGo: *emitGo}); err != nil {
			fmt.Fprintln(os.Stderr, "inkdown:", err)
			return 1
		}
		return 0

	case "check":
		fs := flag.NewFlagSet("check", flag.ExitOnError)
		fs.Parse(rest)
		file, ok := oneFile(fs)
		if !ok {
			return 2
		}
		if err := driver.Check(file); err != nil {
			fmt.Fprintln(os.Stderr, "inkdown:", err)
			return 1
		}
		return 0

	case "help", "-h", "--help":
		fmt.Print(usage)
		return 0
	}

	fmt.Fprintf(os.Stderr, "inkdown: unknown command %q\n\n%s", cmd, usage)
	return 2
}

func oneFile(fs *flag.FlagSet) (string, bool) {
	if fs.NArg() != 1 {
		fmt.Fprint(os.Stderr, "inkdown: expected exactly one program file\n\n"+usage)
		return "", false
	}
	return fs.Arg(0), true
}
