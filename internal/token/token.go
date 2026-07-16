// Package token defines the lexical tokens of Inkdown, source positions, and
// the diagnostic error type shared by all compiler stages.
package token

import "fmt"

// Kind identifies a class of token.
type Kind int

const (
	EOF     Kind = iota
	NEWLINE      // statement terminator: '\n' or ';'

	IDENT
	INT    // integer literal
	FLOAT  // float literal
	STRING // string literal (Lit holds the decoded value)

	// Keywords.
	FUNC
	RECORD
	RETURN
	LET
	VAR
	IF
	ELSE
	WHILE
	FOR
	IN
	BREAK
	CONTINUE
	AND
	OR
	NOT
	TRUE
	FALSE

	// Delimiters.
	LPAREN   // (
	RPAREN   // )
	LBRACE   // {
	RBRACE   // }
	LBRACKET // [
	RBRACKET // ]
	COMMA    // ,
	COLON    // :
	ARROW    // ->
	DOT      // . (namespaced builtin calls: http.get)

	// Operators.
	ASSIGN         // =
	PLUS           // +
	MINUS          // -
	STAR           // *
	SLASH          // /
	PERCENT        // %
	PLUS_ASSIGN    // +=
	MINUS_ASSIGN   // -=
	STAR_ASSIGN    // *=
	SLASH_ASSIGN   // /=
	PERCENT_ASSIGN // %=
	EQ             // ==
	NEQ            // !=
	LT             // <
	LE             // <=
	GT             // >
	GE             // >=
)

var kindNames = map[Kind]string{
	EOF:     "end of file",
	NEWLINE: "newline",
	IDENT:   "identifier",
	INT:     "integer literal",
	FLOAT:   "float literal",
	STRING:  "string literal",

	FUNC: "'func'", RECORD: "'record'", RETURN: "'return'", LET: "'let'", VAR: "'var'",
	IF: "'if'", ELSE: "'else'", WHILE: "'while'", FOR: "'for'", IN: "'in'",
	BREAK: "'break'", CONTINUE: "'continue'",
	AND: "'and'", OR: "'or'", NOT: "'not'", TRUE: "'true'", FALSE: "'false'",

	LPAREN: "'('", RPAREN: "')'", LBRACE: "'{'", RBRACE: "'}'",
	LBRACKET: "'['", RBRACKET: "']'", COMMA: "','", COLON: "':'", ARROW: "'->'",
	DOT: "'.'",

	ASSIGN: "'='", PLUS: "'+'", MINUS: "'-'", STAR: "'*'", SLASH: "'/'",
	PERCENT: "'%'", PLUS_ASSIGN: "'+='", MINUS_ASSIGN: "'-='",
	STAR_ASSIGN: "'*='", SLASH_ASSIGN: "'/='", PERCENT_ASSIGN: "'%='",
	EQ: "'=='", NEQ: "'!='", LT: "'<'", LE: "'<='", GT: "'>'", GE: "'>='",
}

func (k Kind) String() string {
	if s, ok := kindNames[k]; ok {
		return s
	}
	return fmt.Sprintf("token(%d)", int(k))
}

// Keywords maps keyword spellings to their token kinds.
var Keywords = map[string]Kind{
	"func": FUNC, "record": RECORD, "return": RETURN, "let": LET, "var": VAR,
	"if": IF, "else": ELSE, "while": WHILE, "for": FOR, "in": IN,
	"break": BREAK, "continue": CONTINUE,
	"and": AND, "or": OR, "not": NOT, "true": TRUE, "false": FALSE,
}

// Pos is a position in the extracted (tangled) source, 1-based. The driver
// translates lines back to the Markdown document when rendering diagnostics.
type Pos struct {
	Line int
	Col  int
}

// Token is one lexical token.
type Token struct {
	Kind Kind
	Lit  string // literal text: identifier name, number spelling, decoded string value
	Pos  Pos
}

func (t Token) String() string {
	switch t.Kind {
	case IDENT, INT, FLOAT:
		return fmt.Sprintf("%s %q", t.Kind, t.Lit)
	case STRING:
		return fmt.Sprintf("%s %q", t.Kind, t.Lit)
	default:
		return t.Kind.String()
	}
}

// Error is a positioned diagnostic produced by any compiler stage. Positions
// refer to the extracted source; the driver maps them to the Markdown file.
type Error struct {
	Pos Pos
	Msg string
}

func (e *Error) Error() string {
	return fmt.Sprintf("%d:%d: %s", e.Pos.Line, e.Pos.Col, e.Msg)
}

// Errorf builds a positioned diagnostic.
func Errorf(pos Pos, format string, args ...any) *Error {
	return &Error{Pos: pos, Msg: fmt.Sprintf(format, args...)}
}
