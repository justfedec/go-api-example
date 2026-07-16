package stdlib

import "github.com/justfedec/inkdown/internal/types"

func init() {
	register(
		&Spec{Name: "is_int", Params: []types.Type{types.String}, Ret: types.Bool, GoFunc: "_ink_isInt", Chunk: "proc"},
		&Spec{Name: "is_float", Params: []types.Type{types.String}, Ret: types.Bool, GoFunc: "_ink_isFloat", Chunk: "proc"},
		&Spec{Name: "env", Params: []types.Type{types.String}, Ret: types.String, GoFunc: "_ink_env", Chunk: "proc"},
		&Spec{Name: "exit", Params: []types.Type{types.Int}, GoFunc: "_ink_exit", Chunk: "proc"},
		&Spec{Name: "read_line", Ret: types.String, GoFunc: "_ink_readLine", Chunk: "proc"},
	)

	// eprint is variadic like print, so its checking and lowering stay
	// hand-coded; only its helper lives here.
	ManualChunks["eprint"] = "proc"

	Chunks["proc"] = &Chunk{
		Imports: []string{"bufio", "os", "strings"},
		Src: `func _ink_eprint(args ...any) { fmt.Fprintln(os.Stderr, args...) }

func _ink_isInt(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil
}

func _ink_isFloat(s string) bool {
	_, err := strconv.ParseFloat(s, 64)
	return err == nil
}

func _ink_env(name string) string { return os.Getenv(name) }

func _ink_exit(code int) { os.Exit(code) }

var _ink_stdin = bufio.NewReader(os.Stdin)

// _ink_readLine returns the next stdin line without its terminator, or ""
// at end of input.
func _ink_readLine() string {
	line, _ := _ink_stdin.ReadString('\n')
	return strings.TrimRight(line, "\r\n")
}
`,
	}
}
