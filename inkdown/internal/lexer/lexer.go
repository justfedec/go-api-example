// Package lexer turns Inkdown source text into tokens.
//
// Newlines are statement terminators, so the lexer emits NEWLINE tokens —
// except inside parentheses and brackets (implicit line joining, as in
// Python), and never two in a row. Semicolons are lexed as NEWLINE too.
// Comments run from '#' to the end of the line.
package lexer

import (
	"strings"

	"github.com/justfedec/go-api-example/inkdown/internal/token"
)

// Lex scans src and returns the full token stream, ending with an EOF token.
// It stops at the first lexical error.
func Lex(src string) ([]token.Token, error) {
	l := &lexer{src: src, line: 1, col: 1}
	for {
		tok, err := l.next()
		if err != nil {
			return nil, err
		}
		// Collapse consecutive NEWLINEs and drop leading ones.
		if tok.Kind == token.NEWLINE {
			if len(l.toks) == 0 || l.toks[len(l.toks)-1].Kind == token.NEWLINE {
				continue
			}
		}
		l.toks = append(l.toks, tok)
		if tok.Kind == token.EOF {
			return l.toks, nil
		}
	}
}

type lexer struct {
	src   string
	off   int // current byte offset
	line  int // 1-based line of src[off]
	col   int // 1-based column of src[off]
	depth int // ( and [ nesting; newlines are suppressed when > 0
	toks  []token.Token
}

func (l *lexer) pos() token.Pos { return token.Pos{Line: l.line, Col: l.col} }

func (l *lexer) peek() byte {
	if l.off >= len(l.src) {
		return 0
	}
	return l.src[l.off]
}

func (l *lexer) peekAt(i int) byte {
	if l.off+i >= len(l.src) {
		return 0
	}
	return l.src[l.off+i]
}

func (l *lexer) advance() byte {
	ch := l.src[l.off]
	l.off++
	if ch == '\n' {
		l.line++
		l.col = 1
	} else {
		l.col++
	}
	return ch
}

func (l *lexer) next() (token.Token, error) {
	// Skip spaces, tabs, carriage returns, and comments.
	for {
		switch l.peek() {
		case ' ', '\t', '\r':
			l.advance()
			continue
		case '#':
			for l.peek() != '\n' && l.off < len(l.src) {
				l.advance()
			}
			continue
		}
		break
	}

	pos := l.pos()
	if l.off >= len(l.src) {
		return token.Token{Kind: token.EOF, Pos: pos}, nil
	}

	ch := l.peek()
	switch {
	case ch == '\n' || ch == ';':
		l.advance()
		if l.depth > 0 {
			return l.next()
		}
		return token.Token{Kind: token.NEWLINE, Lit: string(ch), Pos: pos}, nil

	case isIdentStart(ch):
		start := l.off
		for isIdentPart(l.peek()) {
			l.advance()
		}
		lit := l.src[start:l.off]
		if kind, ok := token.Keywords[lit]; ok {
			return token.Token{Kind: kind, Lit: lit, Pos: pos}, nil
		}
		return token.Token{Kind: token.IDENT, Lit: lit, Pos: pos}, nil

	case isDigit(ch):
		return l.number(pos)

	case ch == '"':
		return l.stringLit(pos)
	}

	// Operators and delimiters.
	two := func(kind token.Kind, lit string) (token.Token, error) {
		l.advance()
		l.advance()
		return token.Token{Kind: kind, Lit: lit, Pos: pos}, nil
	}
	one := func(kind token.Kind) (token.Token, error) {
		l.advance()
		return token.Token{Kind: kind, Lit: string(ch), Pos: pos}, nil
	}

	nxt := l.peekAt(1)
	switch ch {
	case '(':
		l.depth++
		return one(token.LPAREN)
	case ')':
		l.depth--
		return one(token.RPAREN)
	case '[':
		l.depth++
		return one(token.LBRACKET)
	case ']':
		l.depth--
		return one(token.RBRACKET)
	case '{':
		return one(token.LBRACE)
	case '}':
		return one(token.RBRACE)
	case ',':
		return one(token.COMMA)
	case ':':
		return one(token.COLON)
	case '+':
		if nxt == '=' {
			return two(token.PLUS_ASSIGN, "+=")
		}
		return one(token.PLUS)
	case '-':
		if nxt == '>' {
			return two(token.ARROW, "->")
		}
		if nxt == '=' {
			return two(token.MINUS_ASSIGN, "-=")
		}
		return one(token.MINUS)
	case '*':
		if nxt == '=' {
			return two(token.STAR_ASSIGN, "*=")
		}
		return one(token.STAR)
	case '/':
		if nxt == '=' {
			return two(token.SLASH_ASSIGN, "/=")
		}
		return one(token.SLASH)
	case '%':
		if nxt == '=' {
			return two(token.PERCENT_ASSIGN, "%=")
		}
		return one(token.PERCENT)
	case '=':
		if nxt == '=' {
			return two(token.EQ, "==")
		}
		return one(token.ASSIGN)
	case '!':
		if nxt == '=' {
			return two(token.NEQ, "!=")
		}
		return token.Token{}, token.Errorf(pos, "unexpected character '!' (use 'not' for negation)")
	case '<':
		if nxt == '=' {
			return two(token.LE, "<=")
		}
		return one(token.LT)
	case '>':
		if nxt == '=' {
			return two(token.GE, ">=")
		}
		return one(token.GT)
	case '&', '|':
		return token.Token{}, token.Errorf(pos, "unexpected character %q (use 'and'/'or' for logic)", string(ch))
	}
	return token.Token{}, token.Errorf(pos, "unexpected character %q", string(ch))
}

func (l *lexer) number(pos token.Pos) (token.Token, error) {
	start := l.off
	for isDigit(l.peek()) {
		l.advance()
	}
	kind := token.INT
	if l.peek() == '.' && isDigit(l.peekAt(1)) {
		kind = token.FLOAT
		l.advance() // '.'
		for isDigit(l.peek()) {
			l.advance()
		}
	}
	lit := l.src[start:l.off]
	if isIdentStart(l.peek()) || l.peek() == '.' {
		return token.Token{}, token.Errorf(pos, "malformed number %q", lit+string(l.peek()))
	}
	return token.Token{Kind: kind, Lit: lit, Pos: pos}, nil
}

func (l *lexer) stringLit(pos token.Pos) (token.Token, error) {
	l.advance() // opening quote
	var b strings.Builder
	for {
		if l.off >= len(l.src) || l.peek() == '\n' {
			return token.Token{}, token.Errorf(pos, "unterminated string literal")
		}
		ch := l.advance()
		switch ch {
		case '"':
			return token.Token{Kind: token.STRING, Lit: b.String(), Pos: pos}, nil
		case '\\':
			if l.off >= len(l.src) || l.peek() == '\n' {
				return token.Token{}, token.Errorf(pos, "unterminated string literal")
			}
			esc := l.advance()
			switch esc {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case 'r':
				b.WriteByte('\r')
			case '\\':
				b.WriteByte('\\')
			case '"':
				b.WriteByte('"')
			default:
				return token.Token{}, token.Errorf(pos, "unknown escape sequence '\\%s'", string(esc))
			}
		default:
			b.WriteByte(ch)
		}
	}
}

func isIdentStart(ch byte) bool {
	return ch == '_' || ('a' <= ch && ch <= 'z') || ('A' <= ch && ch <= 'Z')
}

func isIdentPart(ch byte) bool { return isIdentStart(ch) || isDigit(ch) }

func isDigit(ch byte) bool { return '0' <= ch && ch <= '9' }
