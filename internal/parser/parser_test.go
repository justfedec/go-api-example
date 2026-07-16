package parser

import (
	"strings"
	"testing"

	"github.com/justfedec/inkdown/internal/ast"
	"github.com/justfedec/inkdown/internal/token"
)

func TestParseValid(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			"declarations",
			"let x = 5\nvar y: [int] = [1, 2,]\nlet s = \"hi\"",
			`(program (let x 5) (var y:[int] (list 1 2)) (let s "hi"))`,
		},
		{
			"precedence",
			"let b = 1 + 2 * 3 == 7 and not false or true",
			`(program (let b (or (and (== (+ 1 (* 2 3)) 7) (not false)) true)))`,
		},
		{
			"unary and grouping",
			"let a = -x * (y + z)\nlet c = - -w",
			`(program (let a (* (neg x) (+ y z))) (let c (neg (neg w))))`,
		},
		{
			"comparison chain is left associative",
			"let a = 1 < 2 == true",
			`(program (let a (== (< 1 2) true)))`,
		},
		{
			"calls and indexing",
			"print(f(1, 2), xs[0][i + 1], [])",
			`(program (expr (call print (call f 1 2) (index (index xs 0) (+ i 1)) (list))))`,
		},
		{
			"multi line call and list",
			"f(1,\n   2)\nlet xs = [\n  1,\n  2,\n]",
			`(program (expr (call f 1 2)) (let xs (list 1 2)))`,
		},
		{
			"assignments",
			"x = 1\nxs[i] = 2\nn += 3\ngrid[i][j] *= 4",
			`(program (= x 1) (= (index xs i) 2) (+= n 3) (*= (index (index grid i) j) 4))`,
		},
		{
			"if else chain with newline before else",
			"if a {\n  f()\n}\nelse if b {\n  g()\n} else {\n  h()\n}",
			`(program (if a (block (expr (call f))) (if b (block (expr (call g))) (block (expr (call h))))))`,
		},
		{
			"if without else keeps following statement",
			"if a {\n  f()\n}\ng()",
			`(program (if a (block (expr (call f)))) (expr (call g)))`,
		},
		{
			"loops",
			"while n > 0 {\n  n -= 1\n  if n == 3 { continue }\n  if n == 1 { break }\n}\nfor x in range(0, 3) {\n  print(x)\n}",
			`(program (while (> n 0) (block (-= n 1) (if (== n 3) (block (continue))) (if (== n 1) (block (break))))) (for x (call range 0 3) (block (expr (call print x)))))`,
		},
		{
			"function declaration",
			"func fib(n: int) -> int {\n  if n < 2 { return n }\n  return fib(n - 1) + fib(n - 2)\n}\nfunc log(msg: string, xs: [[float]]) {\n  return\n}",
			`(program (func fib ((n int)) -> int (block (if (< n 2) (block (return n))) (return (+ (call fib (- n 1)) (call fib (- n 2)))))) (func log ((msg string) (xs [[float]])) (block (return))))`,
		},
		{
			"semicolons as terminators",
			"let a = 1; let b = 2",
			`(program (let a 1) (let b 2))`,
		},
		{
			"namespaced calls",
			"let r = http.get(\"u\")\nhttp.respond(r, 200, json.escape(s))",
			`(program (let r (call http.get "u")) (expr (call http.respond r 200 (call json.escape s))))`,
		},
		{
			"namespaced call composes with indexing and args",
			"let t = json.get(http.text(rs[0]), \"a.b\")",
			`(program (let t (call json.get (call http.text (index rs 0)) "a.b")))`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			prog, err := Parse(tc.src)
			if err != nil {
				t.Fatalf("Parse error: %v", err)
			}
			if got := ast.Dump(prog); got != tc.want {
				t.Errorf("dump mismatch\n got: %s\nwant: %s", got, tc.want)
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
	cases := []struct {
		name    string
		src     string
		wantMsg string
		line    int
		col     int
	}{
		{"func in block", "if a {\n  func f() {}\n}", "functions can only be declared at the top level", 2, 3},
		{"missing close brace", "while a {\n  f()\n", "expected '}', found end of file", 3, 1},
		{"decl without initializer", "let x\n", "needs an initial value", 1, 6},
		{"assign to call", "f(x) = 3\n", "invalid assignment target", 1, 1},
		{"call of non function expr", "xs[0](1)\n", "only named functions can be called", 1, 6},
		{"bad type", "let x: [3] = []\n", "expected a type, found '3'", 1, 9},
		{"missing bracket", "let a = xs[1 = 2\n", "expected ']', found '='", 1, 14},
		{"expression as garbage", "let a = 1 + \n", "expected an expression, found newline", 1, 13},
		{"two statements one line", "f() g()\n", "expected newline after statement, found 'g'", 1, 5},
		{"else without if", "else {\n}\n", "expected an expression, found 'else'", 1, 1},
		{"leading dot number", "let a = .5\n", "expected an expression, found '.'", 1, 9},
		{"dotted name without call", "let x = a.b\n", "expected '(' after 'a.b'", 1, 12},
		{"dot on non identifier", "f().x(1)\n", "'.' is only valid in namespaced calls", 1, 4},
		{"chained dots", "a.b.c(1)\n", "expected '(' after 'a.b'", 1, 4},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse(tc.src)
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
			if te.Pos.Line != tc.line || te.Pos.Col != tc.col {
				t.Errorf("pos = %d:%d, want %d:%d (msg %q)", te.Pos.Line, te.Pos.Col, tc.line, tc.col, te.Msg)
			}
		})
	}
}
