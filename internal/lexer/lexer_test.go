package lexer

import (
	"strings"
	"testing"

	"github.com/justfedec/inkdown/internal/token"
)

func kinds(toks []token.Token) []token.Kind {
	out := make([]token.Kind, len(toks))
	for i, t := range toks {
		out[i] = t.Kind
	}
	return out
}

func equalKinds(a, b []token.Kind) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestLexKindsAndPositions(t *testing.T) {
	src := "let x = 5\nprint(x)\n"
	toks, err := Lex(src)
	if err != nil {
		t.Fatal(err)
	}
	want := []struct {
		kind      token.Kind
		lit       string
		line, col int
	}{
		{token.LET, "let", 1, 1},
		{token.IDENT, "x", 1, 5},
		{token.ASSIGN, "=", 1, 7},
		{token.INT, "5", 1, 9},
		{token.NEWLINE, "\n", 1, 10},
		{token.IDENT, "print", 2, 1},
		{token.LPAREN, "(", 2, 6},
		{token.IDENT, "x", 2, 7},
		{token.RPAREN, ")", 2, 8},
		{token.NEWLINE, "\n", 2, 9},
		{token.EOF, "", 3, 1},
	}
	if len(toks) != len(want) {
		t.Fatalf("got %d tokens %v, want %d", len(toks), toks, len(want))
	}
	for i, w := range want {
		g := toks[i]
		if g.Kind != w.kind || g.Lit != w.lit || g.Pos.Line != w.line || g.Pos.Col != w.col {
			t.Errorf("tok[%d] = %v at %d:%d, want %v %q at %d:%d",
				i, g, g.Pos.Line, g.Pos.Col, w.kind, w.lit, w.line, w.col)
		}
	}
}

func TestLexOperatorsAndArrow(t *testing.T) {
	src := "-> - -= == = != <= < >= > += *= /= %= % * /"
	toks, err := Lex(src)
	if err != nil {
		t.Fatal(err)
	}
	want := []token.Kind{
		token.ARROW, token.MINUS, token.MINUS_ASSIGN, token.EQ, token.ASSIGN,
		token.NEQ, token.LE, token.LT, token.GE, token.GT,
		token.PLUS_ASSIGN, token.STAR_ASSIGN, token.SLASH_ASSIGN,
		token.PERCENT_ASSIGN, token.PERCENT, token.STAR, token.SLASH,
		token.EOF,
	}
	if !equalKinds(kinds(toks), want) {
		t.Errorf("kinds = %v, want %v", kinds(toks), want)
	}
}

func TestLexNewlineHandling(t *testing.T) {
	// Leading and repeated newlines collapse; newlines vanish inside ( and [;
	// ';' acts as a newline.
	src := "\n\na\n\n\nf(1,\n   2)\n[1,\n2]\nb; c\n"
	toks, err := Lex(src)
	if err != nil {
		t.Fatal(err)
	}
	want := []token.Kind{
		token.IDENT, token.NEWLINE,
		token.IDENT, token.LPAREN, token.INT, token.COMMA, token.INT, token.RPAREN, token.NEWLINE,
		token.LBRACKET, token.INT, token.COMMA, token.INT, token.RBRACKET, token.NEWLINE,
		token.IDENT, token.NEWLINE, token.IDENT, token.NEWLINE,
		token.EOF,
	}
	if !equalKinds(kinds(toks), want) {
		t.Errorf("kinds = %v, want %v", kinds(toks), want)
	}
}

func TestLexCommentsAndNumbers(t *testing.T) {
	src := "x = 42 # the answer\ny = 3.14 # pi\n# full line\nz = 0\n"
	toks, err := Lex(src)
	if err != nil {
		t.Fatal(err)
	}
	var lits []string
	for _, tok := range toks {
		if tok.Kind == token.INT || tok.Kind == token.FLOAT {
			lits = append(lits, tok.Lit)
		}
	}
	if got, want := strings.Join(lits, " "), "42 3.14 0"; got != want {
		t.Errorf("number literals = %q, want %q", got, want)
	}
}

func TestLexStrings(t *testing.T) {
	toks, err := Lex(`m = "a\tb\n\"quoted\"\\"`)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := toks[2].Lit, "a\tb\n\"quoted\"\\"; got != want {
		t.Errorf("decoded string = %q, want %q", got, want)
	}
}

func TestLexKeywordsVsIdents(t *testing.T) {
	toks, err := Lex("for x in xs and not while0 iff")
	if err != nil {
		t.Fatal(err)
	}
	want := []token.Kind{
		token.FOR, token.IDENT, token.IN, token.IDENT, token.AND, token.NOT,
		token.IDENT, token.IDENT, token.EOF,
	}
	if !equalKinds(kinds(toks), want) {
		t.Errorf("kinds = %v, want %v", kinds(toks), want)
	}
}

func TestLexErrors(t *testing.T) {
	cases := []struct {
		src     string
		wantMsg string
		line    int
		col     int
	}{
		{`x = "unterminated`, "unterminated string literal", 1, 5},
		{"x = \"bad\nescape\"", "unterminated string literal", 1, 5},
		{`x = "\q"`, `unknown escape`, 1, 5},
		{"x = 1.", "malformed number", 1, 5},
		{"x = 12abc", "malformed number", 1, 5},
		{"x = y & z", "unexpected character", 1, 7},
		{"x = !y", "unexpected character '!'", 1, 5},
		{"\n\n  ?", "unexpected character", 3, 3},
	}
	for _, tc := range cases {
		_, err := Lex(tc.src)
		if err == nil {
			t.Errorf("Lex(%q): expected error", tc.src)
			continue
		}
		te, ok := err.(*token.Error)
		if !ok {
			t.Errorf("Lex(%q): error is %T, want *token.Error", tc.src, err)
			continue
		}
		if !strings.Contains(te.Msg, tc.wantMsg) {
			t.Errorf("Lex(%q): msg = %q, want containing %q", tc.src, te.Msg, tc.wantMsg)
		}
		if te.Pos.Line != tc.line || te.Pos.Col != tc.col {
			t.Errorf("Lex(%q): pos = %d:%d, want %d:%d", tc.src, te.Pos.Line, te.Pos.Col, tc.line, tc.col)
		}
	}
}
