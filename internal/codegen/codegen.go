// Package codegen turns a checked Inkdown AST into a Go main package.
//
// The translation is 1:1 where possible: Inkdown blocks become Go blocks,
// while becomes for, and operators keep their meaning. Top-level statements
// form the body of main; top-level declarations become package variables so
// functions can see them. Every statement is preceded by a //line directive
// pointing at the original Markdown file, so runtime panics are reported
// against the document the user wrote.
package codegen

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/justfedec/inkdown/internal/ast"
	"github.com/justfedec/inkdown/internal/stdlib"
	"github.com/justfedec/inkdown/internal/token"
	"github.com/justfedec/inkdown/internal/types"
)

// Generate emits the Go source for a checked program. srcName is the
// Markdown file name used in //line directives and mdLine translates
// extracted line numbers to Markdown line numbers.
func Generate(prog *ast.Program, srcName string, mdLine func(int) int) []byte {
	g := &gen{srcName: srcName, mdLine: mdLine}
	g.program(prog)
	return []byte(g.out.String())
}

type gen struct {
	out     strings.Builder
	indent  int
	srcName string
	mdLine  func(int) int
}

// ------------------------------------------------------------------- writing

func (g *gen) line(s string, args ...any) {
	g.out.WriteString(strings.Repeat("\t", g.indent))
	fmt.Fprintf(&g.out, s, args...)
	g.out.WriteByte('\n')
}

func (g *gen) raw(s string) { g.out.WriteString(s) }

// directive emits a //line directive (which must start at column 0) mapping
// the next emitted line to the Markdown document.
func (g *gen) directive(pos token.Pos) {
	fmt.Fprintf(&g.out, "//line %s:%d\n", g.srcName, g.mdLine(pos.Line))
}

// ------------------------------------------------------------- chunk usage

// collectChunks walks the program and reports which stdlib prelude chunks the
// emitted code needs: one for every table (or chunk-dependent manual) builtin
// called, and one for every handle type mentioned in a type annotation —
// annotations are the only way a handle type can appear without a
// constructing builtin call.
func collectChunks(prog *ast.Program) map[string]bool {
	used := map[string]bool{}

	var walkType func(t ast.TypeExpr)
	walkType = func(t ast.TypeExpr) {
		switch t := t.(type) {
		case *ast.NamedType:
			if chunk, ok := stdlib.TypeChunks[t.Name]; ok {
				used[chunk] = true
			}
		case *ast.ListType:
			walkType(t.Elem)
		}
	}

	var walkExpr func(e ast.Expr)
	walkExpr = func(e ast.Expr) {
		switch e := e.(type) {
		case *ast.CallExpr:
			if spec, ok := stdlib.Specs[e.Fun.Name]; ok {
				used[spec.Chunk] = true
			} else if chunk, ok := stdlib.ManualChunks[e.Fun.Name]; ok {
				used[chunk] = true
			}
			for _, a := range e.Args {
				walkExpr(a)
			}
		case *ast.UnaryExpr:
			walkExpr(e.X)
		case *ast.BinaryExpr:
			walkExpr(e.X)
			walkExpr(e.Y)
		case *ast.IndexExpr:
			walkExpr(e.X)
			walkExpr(e.Index)
		case *ast.ListLit:
			for _, el := range e.Elems {
				walkExpr(el)
			}
		}
	}

	var walkStmt func(s ast.Stmt)
	walkBlock := func(b *ast.Block) {
		for _, s := range b.Stmts {
			walkStmt(s)
		}
	}
	walkStmt = func(s ast.Stmt) {
		switch s := s.(type) {
		case *ast.DeclStmt:
			if s.Ann != nil {
				walkType(s.Ann)
			}
			walkExpr(s.Value)
		case *ast.AssignStmt:
			walkExpr(s.Target)
			walkExpr(s.Value)
		case *ast.IfStmt:
			walkExpr(s.Cond)
			walkBlock(s.Then)
			switch e := s.Else.(type) {
			case *ast.IfStmt:
				walkStmt(e)
			case *ast.Block:
				walkBlock(e)
			}
		case *ast.WhileStmt:
			walkExpr(s.Cond)
			walkBlock(s.Body)
		case *ast.ForStmt:
			walkExpr(s.Iter)
			walkBlock(s.Body)
		case *ast.ReturnStmt:
			if s.Value != nil {
				walkExpr(s.Value)
			}
		case *ast.ExprStmt:
			walkExpr(s.X)
		case *ast.FuncDecl:
			for _, p := range s.Params {
				walkType(p.Type)
			}
			if s.Ret != nil {
				walkType(s.Ret)
			}
			walkBlock(s.Body)
		}
	}
	for _, s := range prog.Stmts {
		walkStmt(s)
	}
	return used
}

// chunkImports returns the sorted union of the used chunks' imports, minus
// the unconditional fmt and strconv.
func chunkImports(used map[string]bool) []string {
	seen := map[string]bool{"fmt": true, "strconv": true}
	var imports []string
	for key := range used {
		for _, imp := range stdlib.Chunks[key].Imports {
			if !seen[imp] {
				seen[imp] = true
				imports = append(imports, imp)
			}
		}
	}
	sort.Strings(imports)
	return imports
}

// ------------------------------------------------------------------ prelude

const prelude = `func _ink_range(a, b int) []int {
	if b <= a {
		return nil
	}
	xs := make([]int, 0, b-a)
	for i := a; i < b; i++ {
		xs = append(xs, i)
	}
	return xs
}

func _ink_parseInt(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		panic("int(): cannot parse " + strconv.Quote(s) + " as an integer")
	}
	return n
}

func _ink_parseFloat(s string) float64 {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		panic("float(): cannot parse " + strconv.Quote(s) + " as a number")
	}
	return f
}
`

func (g *gen) program(prog *ast.Program) {
	chunks := collectChunks(prog)

	g.line("// Code generated by inkdown from %s. DO NOT EDIT.", g.srcName)
	g.line("package main")
	g.line("")
	imports := append([]string{"fmt", "strconv"}, chunkImports(chunks)...)
	sort.Strings(imports)
	g.line("import (")
	for _, imp := range imports {
		g.line("\t%q", imp)
	}
	g.line(")")
	g.line("")
	// Keep fmt and strconv used even when the program needs neither. Chunk
	// imports need no guards: each is referenced by its own chunk's body.
	g.line("var _ = fmt.Println")
	g.line("var _ = strconv.Itoa")
	g.line("")
	g.raw(prelude)
	for _, key := range stdlib.ChunkOrder {
		if chunks[key] {
			g.line("")
			g.raw(stdlib.Chunks[key].Src)
		}
	}

	// Package-level variables for top-level declarations.
	var wroteGlobal bool
	for _, s := range prog.Stmts {
		if d, ok := s.(*ast.DeclStmt); ok && d.Global {
			if !wroteGlobal {
				g.line("")
				wroteGlobal = true
			}
			g.line("var %s %s", sanitize(d.Name), goType(d.VarT))
		}
	}

	for _, s := range prog.Stmts {
		if fn, ok := s.(*ast.FuncDecl); ok {
			g.line("")
			g.funcDecl(fn)
		}
	}

	g.line("")
	g.directive(prog.Pos())
	g.line("func main() {")
	g.indent++
	for _, s := range prog.Stmts {
		if _, ok := s.(*ast.FuncDecl); ok {
			continue
		}
		g.stmt(s)
	}
	g.indent--
	g.line("}")
}

func (g *gen) funcDecl(fn *ast.FuncDecl) {
	var params []string
	for i, p := range fn.Params {
		// The checker resolved parameter types; recover them from the
		// annotation syntax to avoid carrying a second table around.
		params = append(params, sanitize(p.Name)+" "+goTypeExpr(fn.Params[i].Type))
	}
	ret := ""
	if fn.Ret != nil {
		ret = " " + goTypeExpr(fn.Ret)
	}
	g.directive(fn.Pos())
	g.line("func %s(%s)%s {", sanitize(fn.Name), strings.Join(params, ", "), ret)
	g.indent++
	for _, s := range fn.Body.Stmts {
		g.stmt(s)
	}
	g.indent--
	g.line("}")
}

// ---------------------------------------------------------------- statements

func (g *gen) stmt(s ast.Stmt) {
	g.directive(s.Pos())
	switch s := s.(type) {
	case *ast.DeclStmt:
		name := sanitize(s.Name)
		switch {
		case s.Global:
			g.line("%s = %s", name, g.exprString(s.Value, 0))
		case s.Ann != nil:
			g.line("var %s %s = %s", name, goType(s.VarT), g.exprString(s.Value, 0))
		default:
			g.line("%s := %s", name, g.exprString(s.Value, 0))
		}
		if s.Unused {
			g.line("_ = %s", name)
		}

	case *ast.AssignStmt:
		g.line("%s %s %s", g.exprString(s.Target, precPostfix), goAssignOp[s.Op], g.exprString(s.Value, 0))

	case *ast.IfStmt:
		g.ifStmt(s)

	case *ast.WhileStmt:
		g.line("for %s {", g.exprString(s.Cond, 0))
		g.block(s.Body)
		g.line("}")

	case *ast.ForStmt:
		if s.Unused {
			g.line("for range %s {", g.exprString(s.Iter, 0))
		} else {
			g.line("for _, %s := range %s {", sanitize(s.Var), g.exprString(s.Iter, 0))
		}
		g.block(s.Body)
		g.line("}")

	case *ast.ReturnStmt:
		if s.Value == nil {
			g.line("return")
		} else {
			g.line("return %s", g.exprString(s.Value, 0))
		}

	case *ast.BreakStmt:
		g.line("break")
	case *ast.ContinueStmt:
		g.line("continue")

	case *ast.ExprStmt:
		call := s.X.(*ast.CallExpr)
		if call.Fun.Name == "push" {
			target := g.exprString(call.Args[0], precPostfix)
			g.line("%s = append(%s, %s)", target, target, g.exprString(call.Args[1], 0))
			return
		}
		g.line("%s", g.exprString(call, 0))

	default:
		panic(fmt.Sprintf("unhandled statement %T", s))
	}
}

func (g *gen) block(b *ast.Block) {
	g.indent++
	for _, s := range b.Stmts {
		g.stmt(s)
	}
	g.indent--
}

func (g *gen) ifStmt(s *ast.IfStmt) {
	g.line("if %s {", g.exprString(s.Cond, 0))
	g.block(s.Then)
	for s.Else != nil {
		switch e := s.Else.(type) {
		case *ast.IfStmt:
			g.line("} else if %s {", g.exprString(e.Cond, 0))
			g.block(e.Then)
			s = e
		case *ast.Block:
			g.line("} else {")
			g.block(e)
			g.line("}")
			return
		}
	}
	g.line("}")
}

// --------------------------------------------------------------- expressions

// Go operator precedence levels used for minimal parenthesization.
const (
	precOr      = 1 // ||
	precAnd     = 2 // &&
	precCmp     = 3 // == != < <= > >=
	precAdd     = 4 // + -
	precMul     = 5 // * / %
	precUnary   = 6
	precPostfix = 7
)

var goBinOp = map[token.Kind]struct {
	text string
	prec int
}{
	token.OR:      {"||", precOr},
	token.AND:     {"&&", precAnd},
	token.EQ:      {"==", precCmp},
	token.NEQ:     {"!=", precCmp},
	token.LT:      {"<", precCmp},
	token.LE:      {"<=", precCmp},
	token.GT:      {">", precCmp},
	token.GE:      {">=", precCmp},
	token.PLUS:    {"+", precAdd},
	token.MINUS:   {"-", precAdd},
	token.STAR:    {"*", precMul},
	token.SLASH:   {"/", precMul},
	token.PERCENT: {"%", precMul},
}

var goAssignOp = map[token.Kind]string{
	token.ASSIGN: "=", token.PLUS_ASSIGN: "+=", token.MINUS_ASSIGN: "-=",
	token.STAR_ASSIGN: "*=", token.SLASH_ASSIGN: "/=", token.PERCENT_ASSIGN: "%=",
}

func (g *gen) exprString(e ast.Expr, prec int) string {
	var b strings.Builder
	g.expr(&b, e, prec)
	return b.String()
}

// expr writes e into b, adding parentheses when e binds looser than the
// surrounding context prec.
func (g *gen) expr(b *strings.Builder, e ast.Expr, prec int) {
	switch e := e.(type) {
	case *ast.IntLit:
		b.WriteString(e.Raw)
	case *ast.FloatLit:
		b.WriteString(e.Raw)
	case *ast.StringLit:
		b.WriteString(strconv.Quote(e.Value))
	case *ast.BoolLit:
		fmt.Fprintf(b, "%v", e.Value)
	case *ast.Ident:
		b.WriteString(sanitize(e.Name))

	case *ast.ListLit:
		list := e.Type().(*types.List)
		b.WriteString(goType(list))
		b.WriteByte('{')
		for i, el := range e.Elems {
			if i > 0 {
				b.WriteString(", ")
			}
			g.expr(b, el, 0)
		}
		b.WriteByte('}')

	case *ast.IndexExpr:
		g.expr(b, e.X, precPostfix)
		b.WriteByte('[')
		g.expr(b, e.Index, 0)
		b.WriteByte(']')

	case *ast.UnaryExpr:
		// Inkdown's 'not' binds looser than Go's '!', so parenthesize the
		// operand unconditionally; same for '-' (also avoids "--x").
		if e.Op == token.NOT {
			b.WriteString("!(")
		} else {
			b.WriteString("-(")
		}
		g.expr(b, e.X, 0)
		b.WriteByte(')')

	case *ast.BinaryExpr:
		op := goBinOp[e.Op]
		if op.prec < prec {
			b.WriteByte('(')
			g.expr(b, e.X, op.prec)
			fmt.Fprintf(b, " %s ", op.text)
			g.expr(b, e.Y, op.prec+1)
			b.WriteByte(')')
			return
		}
		g.expr(b, e.X, op.prec)
		fmt.Fprintf(b, " %s ", op.text)
		g.expr(b, e.Y, op.prec+1)

	case *ast.CallExpr:
		g.call(b, e, prec)

	default:
		panic(fmt.Sprintf("unhandled expression %T", e))
	}
}

func (g *gen) call(b *strings.Builder, e *ast.CallExpr, prec int) {
	args := func(open string) {
		b.WriteString(open)
		for i, a := range e.Args {
			if i > 0 {
				b.WriteString(", ")
			}
			g.expr(b, a, 0)
		}
		b.WriteByte(')')
	}

	switch e.Fun.Name {
	case "print":
		args("fmt.Println(")
	case "eprint":
		args("_ink_eprint(")
	case "len":
		args("len(")
	case "range":
		args("_ink_range(")

	case "str":
		arg := e.Args[0]
		switch t := arg.Type(); {
		case t.Equal(types.Int):
			b.WriteString("strconv.Itoa(")
			g.expr(b, arg, 0)
			b.WriteByte(')')
		case t.Equal(types.Float):
			b.WriteString("strconv.FormatFloat(")
			g.expr(b, arg, 0)
			b.WriteString(", 'g', -1, 64)")
		case t.Equal(types.Bool):
			b.WriteString("strconv.FormatBool(")
			g.expr(b, arg, 0)
			b.WriteByte(')')
		case t.Equal(types.String):
			g.expr(b, arg, prec) // identity
		default: // lists
			b.WriteString("fmt.Sprint(")
			g.expr(b, arg, 0)
			b.WriteByte(')')
		}

	case "int":
		arg := e.Args[0]
		switch t := arg.Type(); {
		case t.Equal(types.Int):
			g.expr(b, arg, prec) // identity
		case t.Equal(types.Float):
			b.WriteString("int(")
			g.expr(b, arg, 0)
			b.WriteByte(')')
		default: // string
			b.WriteString("_ink_parseInt(")
			g.expr(b, arg, 0)
			b.WriteByte(')')
		}

	case "float":
		arg := e.Args[0]
		switch t := arg.Type(); {
		case t.Equal(types.Float):
			g.expr(b, arg, prec) // identity
		case t.Equal(types.Int):
			b.WriteString("float64(")
			g.expr(b, arg, 0)
			b.WriteByte(')')
		default: // string
			b.WriteString("_ink_parseFloat(")
			g.expr(b, arg, 0)
			b.WriteByte(')')
		}

	default:
		if spec, ok := stdlib.Specs[e.Fun.Name]; ok {
			args(spec.GoFunc + "(")
			return
		}
		args(sanitize(e.Fun.Name) + "(")
	}
}

// --------------------------------------------------------------------- names

// goReserved contains every identifier that must not collide with a user
// name in the generated code: Go keywords, predeclared identifiers, our
// imports, and main/init. Many of these cannot be Inkdown identifiers anyway;
// keeping the full list is harmless and future-proof.
var goReserved = map[string]bool{
	// keywords
	"break": true, "case": true, "chan": true, "const": true, "continue": true,
	"default": true, "defer": true, "else": true, "fallthrough": true,
	"for": true, "func": true, "go": true, "goto": true, "if": true,
	"import": true, "interface": true, "map": true, "package": true,
	"range": true, "return": true, "select": true, "struct": true,
	"switch": true, "type": true, "var": true,
	// predeclared
	"any": true, "bool": true, "byte": true, "comparable": true,
	"complex64": true, "complex128": true, "error": true, "float32": true,
	"float64": true, "int": true, "int8": true, "int16": true, "int32": true,
	"int64": true, "rune": true, "string": true, "uint": true, "uint8": true,
	"uint16": true, "uint32": true, "uint64": true, "uintptr": true,
	"true": true, "false": true, "iota": true, "nil": true,
	"append": true, "cap": true, "clear": true, "close": true,
	"complex": true, "copy": true, "delete": true, "imag": true, "len": true,
	"make": true, "max": true, "min": true, "new": true, "panic": true,
	"print": true, "println": true, "real": true, "recover": true,
	// used by the generated file (fmt/strconv always; the rest when their
	// prelude chunk is emitted — reserving them unconditionally is harmless)
	"fmt": true, "strconv": true, "main": true, "init": true,
	"strings": true, "os": true, "net": true, "http": true, "json": true,
	"bufio": true, "time": true, "io": true, "bytes": true, "utf8": true,
}

// sanitize maps an Inkdown identifier to a safe Go identifier. Appending
// "_" keeps the emitted code readable; a user program declaring both "go"
// and "go_" would collide, which we accept as a v1 limitation.
func sanitize(name string) string {
	if goReserved[name] || strings.HasPrefix(name, "_ink_") {
		return name + "_"
	}
	return name
}

func goType(t types.Type) string {
	switch t := t.(type) {
	case *types.Basic:
		switch t.Kind {
		case types.IntKind:
			return "int"
		case types.FloatKind:
			return "float64"
		case types.StringKind:
			return "string"
		case types.BoolKind:
			return "bool"
		}
	case *types.List:
		return "[]" + goType(t.Elem)
	case *types.Opaque:
		return "*_ink_" + t.Name
	}
	panic(fmt.Sprintf("unhandled type %v", t))
}

// goTypeExpr maps type syntax (already validated by the checker) to Go.
func goTypeExpr(t ast.TypeExpr) string {
	switch t := t.(type) {
	case *ast.NamedType:
		if t.Name == "float" {
			return "float64"
		}
		if stdlib.Types[t.Name] != nil {
			return "*_ink_" + t.Name
		}
		return t.Name
	case *ast.ListType:
		return "[]" + goTypeExpr(t.Elem)
	}
	panic("unreachable")
}
