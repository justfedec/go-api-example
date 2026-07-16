// Package stdlib is the single registry for Inkdown's table-driven builtins.
//
// v1's builtins (print, len, range, push, str, int, float) have polymorphic
// or effectful shapes and stay hand-coded in the checker and code generator.
// Everything added since is monomorphic and registered here once: the checker
// derives reservation and validates calls from Spec, and the code generator
// lowers every table builtin to its GoFunc prelude helper and emits only the
// prelude chunks (and their imports) that a program actually uses.
package stdlib

import "github.com/justfedec/inkdown/internal/types"

// Spec describes one table builtin.
type Spec struct {
	Name    string       // "split" or "http.get"
	Params  []types.Type // full parameter list
	MinArgs int          // minimum arity for optional trailing params; 0 means len(Params)
	Ret     types.Type   // nil: no value
	GoFunc  string       // prelude helper the call lowers to, e.g. "_ink_split"
	Chunk   string       // prelude chunk that defines GoFunc
}

// Min returns the minimum number of arguments for the spec.
func (s *Spec) Min() int {
	if s.MinArgs > 0 {
		return s.MinArgs
	}
	return len(s.Params)
}

// Chunk is a prelude fragment: helper source plus the imports its body needs.
// fmt and strconv are always imported by the generated file and need not be
// listed.
type Chunk struct {
	Imports []string
	Src     string
}

// Roots are the dot-namespace prefixes. They are reserved identifiers.
var Roots = map[string]bool{}

// Types are the opaque handle types, by name. Reserved identifiers, usable in
// type annotations.
var Types = map[string]*types.Opaque{}

// TypeChunks maps a handle type name to the chunk defining its Go struct, so
// a program that only *annotates* the type (without calling a constructor
// builtin) still gets the definition emitted.
var TypeChunks = map[string]string{}

// Specs indexes every table builtin by name.
var Specs = map[string]*Spec{}

// ManualChunks maps hand-coded (non-table) builtins to the chunk their
// lowering depends on.
var ManualChunks = map[string]string{}

// Chunks holds the prelude fragments by key; ChunkOrder fixes emission order.
var Chunks = map[string]*Chunk{}
var ChunkOrder = []string{"strings", "proc", "httpserver", "httpclient", "json", "llm"}

func register(specs ...*Spec) {
	for _, s := range specs {
		Specs[s.Name] = s
	}
}
