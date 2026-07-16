package check

import (
	"strings"
	"testing"

	"github.com/justfedec/inkdown/internal/ast"
	"github.com/justfedec/inkdown/internal/parser"
	"github.com/justfedec/inkdown/internal/token"
)

func mustParse(t *testing.T, src string) *ast.Program {
	t.Helper()
	prog, err := parser.Parse(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return prog
}

func TestCheckValidPrograms(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{"hello", `print("hello")`},
		{"fibonacci", `
func fib(n: int) -> int {
  if n < 2 { return n }
  return fib(n - 1) + fib(n - 2)
}
for i in range(0, 10) {
  print("fib(" + str(i) + ") =", fib(i))
}
`},
		{"globals visible in functions", `
let greeting = "Hello"
func greet(name: string) {
  print(greeting + ", " + name)
}
greet("Ana")
`},
		{"function order does not matter", `
func a() -> int { return b() + 1 }
func b() -> int { return 41 }
print(a())
`},
		{"lists and mutation", `
var xs: [int] = []
push(xs, 1)
push(xs, 2)
xs[0] = 10
xs[1] += 5
let grid = [[1, 2], [3, 4]]
print(xs[0], grid[1][0], len(xs), len("abc"))
`},
		{"shadowing in nested blocks", `
let x = 1
if true {
  let x = "inner"
  print(x)
}
print(x)
`},
		{"conversions and floats", `
let a = float(3) / 2.0
let b = int("42") + int(2.9)
let c = str(a) + " " + str(b) + str(true) + str([1, 2])
print(c)
`},
		{"while with break and continue", `
var n = 10
while true {
  n -= 1
  if n % 2 == 0 { continue }
  if n < 3 { break }
}
print(n)
`},
		{"discarding a user function result", `
func f() -> int { return 1 }
f()
`},
		{"if else chain terminates", `
func sign(n: int) -> int {
  if n > 0 {
    return 1
  } else if n < 0 {
    return -1
  } else {
    return 0
  }
}
print(sign(-5))
`},
		{"logic operators", `
let p = true and not false or 1 < 2
if p { print("yes") }
`},
		{"empty list through function param", `
func total(xs: [int]) -> int {
  var t = 0
  for x in xs { t += x }
  return t
}
print(total([]))
print(total([1, 2, 3]))
`},
		{"records: construct, select, mutate", `
record Todo {
  id: int
  title: string
  done: bool
}
var t = Todo(id: 1, title: "x", done: false)
t.done = not t.done
print(t.id, t.title, t.done)
`},
		{"records: nested, lists, and mutual references", `
record A { b: B }
record B { n: int }
func first(xs: [A]) -> int { return xs[0].b.n }
var as: [A] = []
push(as, A(b: B(n: 7)))
print(first(as))
`},
		{"records: recursive type declaration", `
record Node { value: int, next: Node }
func head(n: Node) -> int { return n.value }
`},
		{"records: field takes an empty list via constructor", `
record Bag { items: [int] }
let b = Bag(items: [])
print(len(b.items))
`},
		{"error binding on int and json", `
let n, err = int("42")
if err != "" { print(err) }
print(n)
let doc = "{}"
let v, gerr = json.get(doc, "x")
print(v, gerr)
`},
		{"error binding on float", `
var f, e = float("3.5")
print(f, e)
`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := Check(mustParse(t, tc.src)); err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestCheckErrors(t *testing.T) {
	cases := []struct {
		name    string
		src     string
		wantMsg string
	}{
		{"undefined variable", `print(x)`, "'x' is not defined"},
		{"undefined function", `f()`, "'f' is not defined"},
		{"use before declaration at top level", "print(g)\nlet g = 1", "'g' is not defined"},
		{"redeclaration", "let x = 1\nlet x = 2", "'x' is already declared"},
		{"redeclare builtin", `let print = 1`, "cannot redeclare builtin 'print'"},
		{"assign to let", "let x = 1\nx = 2", "declared with 'let' and cannot be reassigned"},
		{"assign to parameter", "func f(n: int) { n = 2 }", "parameter 'n' cannot be reassigned"},
		{"assign to loop variable", "for i in range(0, 3) { i = 5 }", "loop variable 'i' cannot be reassigned"},
		{"assign to function", "func f() {}\nf = 1", "cannot assign to function 'f'"},
		{"assign to builtin", `len = 1`, "cannot assign to builtin 'len'"},
		{"mutate let list", "let xs = [1]\nxs[0] = 2", "declared with 'let' and cannot be modified"},
		{"push into let list", "let xs = [1]\npush(xs, 2)", "cannot be modified by push()"},
		{"annotation mismatch", `let x: int = "a"`, "cannot initialize 'x' (declared int) with a string value"},
		{"mixed arithmetic", `let x = 1 + 1.5`, "mismatched operand types (int and float)"},
		{"modulo on floats", `let x = 1.0 % 2.0`, "'%' needs int operands"},
		{"plus on bools", `let x = true + false`, "'+' works on int, float, or string"},
		{"and on ints", `let x = 1 and 2`, "'and' needs bool operands"},
		{"not on int", `let x = not 1`, "'not' needs a bool operand"},
		{"minus on string", `let x = -"a"`, "unary '-' needs an int or float operand"},
		{"condition not bool", `if 1 { print(1) }`, "'if' condition must be bool"},
		{"while condition not bool", `while "x" { print(1) }`, "'while' condition must be bool"},
		{"for over non list", `for x in 5 { print(x) }`, "'for' needs a list to iterate over, got int"},
		{"index non list", "let a = 5\nlet b = a[0]", "cannot index a value of type int"},
		{"index string", `let b = "abc"[0]`, "strings cannot be indexed"},
		{"non int index", "let xs = [1]\nlet b = xs[true]", "list index must be int, got bool"},
		{"compare lists", `let b = [1] == [1]`, "values of type [int] cannot be compared"},
		{"order bools", `let b = true < false`, "values of type bool cannot be ordered"},
		{"empty list without context", `let xs = []`, "cannot infer the type of an empty list"},
		{"heterogeneous list", `let xs = [1, "a"]`, "list elements must all have the same type"},
		{"call arity", "func f(n: int) {}\nf(1, 2)", "function 'f' takes 1 argument(s), got 2"},
		{"call argument type", "func f(n: int) {}\nf(\"a\")", "argument 1 of 'f' must be int, got string"},
		{"call a variable", "let x = 1\nx()", "'x' is a variable, not a function"},
		{"function as value", "func f() {}\nlet g = f", "function 'f' can only be called"},
		{"builtin as value", `let g = str`, "builtin 'str' can only be called"},
		{"string as function", `let s = string(5)`, "'string' is a type, not a function (use str(x)"},
		{"len arity", `let n = len("a", "b")`, "len() takes 1 argument, got 2"},
		{"len of int", `let n = len(5)`, "len() needs a string or a list"},
		{"range bounds", `for i in range(0, "x") { print(i) }`, "range() bounds must be int"},
		{"push rvalue", `push(range(0, 1), 1)`, "must be a list variable"},
		{"push wrong element", "var xs = [1]\npush(xs, \"a\")", "cannot push a string value into a [int]"},
		{"push non list", "var n = 1\npush(n, 2)", "push() needs a list as its first argument"},
		{"push target index with call", "func f() -> int { return 0 }\nvar g = [[1]]\npush(g[f()], 2)", "must not contain function calls"},
		{"int of bool", `let n = int(true)`, "int() takes an int, float, or string, got bool"},
		{"unused builtin result", "let xs = [1]\nlen(xs)", "result of len(...) is unused"},
		{"non call statement", `1 + 2`, "only calls can be used as statements"},
		{"void call in expression", "func f() {}\nlet x = f()", "f(...) does not return a value"},
		{"print in expression", `let x = print(1)`, "print(...) does not return a value"},
		{"return at top level", `return 1`, "'return' is not allowed at the top level"},
		{"bare return in typed function", "func f() -> int { return }", "missing return value"},
		{"value return in void function", "func f() { return 1 }", "does not declare a return type"},
		{"wrong return type", `func f() -> int { return "a" }`, "returns int, but this returns a string"},
		{"missing return", "func f() -> int { print(1) }", "missing return"},
		{"missing final else", "func f(n: int) -> int {\n  if n > 0 { return 1 } else if n < 0 { return -1 }\n}", "missing return"},
		{"break outside loop", `break`, "'break' is only allowed inside a loop"},
		{"continue outside loop", "func f() { continue }", "'continue' is only allowed inside a loop"},
		{"unknown type", `let x: banana = 1`, "unknown type 'banana'"},
		{"str as type", `let s: str = "x"`, "the type is called 'string'"},
		{"duplicate function", "func f() {}\nfunc f() {}", "'f' is already declared"},
		{"global vs function collision", "func f() {}\nlet f = 1", "'f' is already declared"},
		{"record missing field", "record P { x: int, y: int }\nlet p = P(x: 1)", "missing the field 'y'"},
		{"record unknown field", "record P { x: int }\nlet p = P(x: 1, z: 2)", "has no field 'z'"},
		{"record field set twice", "record P { x: int }\nlet p = P(x: 1, x: 2)", "field 'x' is given twice"},
		{"record positional args", "record P { x: int }\nlet p = P(1)", "takes named arguments"},
		{"record field type mismatch", "record P { x: int }\nlet p = P(x: \"a\")", "field 'x' of 'P' is int, got a string value"},
		{"record field named String", "record P { String: int }", "cannot be named 'String'"},
		{"record duplicate field", "record P { x: int, x: int }", "already has a field 'x'"},
		{"select on non record", "let n = 5\nprint(n.x)", "values of type int have no fields"},
		{"select on handle", "let s = http.serve(0)\nprint(s.port)", "have no fields"},
		{"unknown field access", "record P { x: int }\nlet p = P(x: 1)\nprint(p.y)", "has no field 'y'"},
		{"record used as value", "record P { x: int }\nlet q = P", "is a record type; construct a value"},
		{"record vs global collision", "record P { x: int }\nlet P = 1", "cannot redeclare record type 'P'"},
		{"record vs builtin collision", "record len { x: int }", "cannot redeclare builtin 'len'"},
		{"duplicate record", "record P { x: int }\nrecord P { y: int }", "record 'P' is already declared"},
		{"named args to function", "func f(n: int) {}\nf(n: 1)", "named arguments are only for record constructors"},
		{"method-style call on record", "record P { x: int }\nvar p = P(x: 1)\np.x(1)", "records have no methods"},
		{"unused constructor result", "record P { x: int }\nP(x: 1)", "result of P(...) is unused"},
		{"record constructor result discarded ok is error", "record P { x: int }\nfunc f() { P(x: 1) }", "result of P(...) is unused"},
		{"err binding on non-fallible builtin", `let n, err = len("a")`, "'len' cannot fail"},
		{"err binding on user function", "func f() -> int { return 1 }\nlet n, err = f()", "'f' cannot fail"},
		{"err binding on non-call", `let n, err = 5`, "needs a call to a fallible builtin"},
		{"err binding int on non-string", `let n, err = int(3.5)`, "can only fail on a string argument"},
		{"err binding shadow collision", "let err = 1\nlet n, err = int(\"5\")", "'err' is already declared"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := Check(mustParse(t, tc.src))
			if err == nil {
				t.Fatal("expected error, got none")
			}
			te, ok := err.(*token.Error)
			if !ok {
				t.Fatalf("error is %T (%v), want *token.Error", err, err)
			}
			if !strings.Contains(te.Msg, tc.wantMsg) {
				t.Errorf("msg = %q, want containing %q", te.Msg, tc.wantMsg)
			}
			if te.Pos.Line == 0 || te.Pos.Col == 0 {
				t.Errorf("error has no position: %v", te)
			}
		})
	}
}

func TestCheckAnnotations(t *testing.T) {
	src := `
let g = 1
if true {
  let local = 2
  let unusedLocal = 3
  print(local)
}
for i in range(0, 3) { print(i) }
for j in range(0, 3) { print("x") }
`
	prog := mustParse(t, src)
	if err := Check(prog); err != nil {
		t.Fatal(err)
	}

	g := prog.Stmts[0].(*ast.DeclStmt)
	if !g.Global {
		t.Error("top-level declaration not marked Global")
	}
	if g.VarT == nil || g.VarT.String() != "int" {
		t.Errorf("g.VarT = %v, want int", g.VarT)
	}
	if g.Unused {
		t.Error("global must never be marked Unused (package vars are exempt in Go)")
	}

	ifStmt := prog.Stmts[1].(*ast.IfStmt)
	local := ifStmt.Then.Stmts[0].(*ast.DeclStmt)
	unused := ifStmt.Then.Stmts[1].(*ast.DeclStmt)
	if local.Global || unused.Global {
		t.Error("block-scoped declarations must not be Global")
	}
	if local.Unused {
		t.Error("read local wrongly marked Unused")
	}
	if !unused.Unused {
		t.Error("unread local not marked Unused")
	}

	forUsed := prog.Stmts[2].(*ast.ForStmt)
	forUnused := prog.Stmts[3].(*ast.ForStmt)
	if forUsed.Unused {
		t.Error("read loop variable wrongly marked Unused")
	}
	if !forUnused.Unused {
		t.Error("unread loop variable not marked Unused")
	}
}
