// Package parser builds an AST from a token stream by recursive descent.
// It stops at the first syntax error.
package parser

import (
	"github.com/justfedec/go-api-example/inkdown/internal/ast"
	"github.com/justfedec/go-api-example/inkdown/internal/lexer"
	"github.com/justfedec/go-api-example/inkdown/internal/token"
)

// Parse lexes and parses src into a Program.
func Parse(src string) (prog *ast.Program, err error) {
	toks, err := lexer.Lex(src)
	if err != nil {
		return nil, err
	}
	p := &parser{toks: toks}
	defer func() {
		if r := recover(); r != nil {
			if perr, ok := r.(*token.Error); ok {
				prog, err = nil, perr
				return
			}
			panic(r)
		}
	}()
	return p.parseProgram(), nil
}

type parser struct {
	toks []token.Token
	i    int
}

// --------------------------------------------------------------- token access

func (p *parser) cur() token.Token     { return p.toks[p.i] }
func (p *parser) at(k token.Kind) bool { return p.toks[p.i].Kind == k }

func (p *parser) advance() token.Token {
	t := p.toks[p.i]
	if t.Kind != token.EOF {
		p.i++
	}
	return t
}

func (p *parser) expect(k token.Kind) token.Token {
	if !p.at(k) {
		p.fail("expected %s, found %s", k, p.describeCur())
	}
	return p.advance()
}

func (p *parser) skipNewlines() {
	for p.at(token.NEWLINE) {
		p.advance()
	}
}

func (p *parser) fail(format string, args ...any) {
	panic(token.Errorf(p.cur().Pos, format, args...))
}

func (p *parser) describeCur() string {
	t := p.cur()
	switch t.Kind {
	case token.IDENT:
		return "'" + t.Lit + "'"
	case token.INT, token.FLOAT:
		return "'" + t.Lit + "'"
	default:
		return t.Kind.String()
	}
}

// ------------------------------------------------------------------ structure

func (p *parser) parseProgram() *ast.Program {
	prog := &ast.Program{}
	p.skipNewlines()
	for !p.at(token.EOF) {
		var s ast.Stmt
		if p.at(token.FUNC) {
			s = p.parseFuncDecl()
		} else {
			s = p.parseStatement(true)
		}
		prog.Stmts = append(prog.Stmts, s)
		p.endOfStatement()
	}
	return prog
}

// endOfStatement consumes the statement terminator: one or more newlines, or
// lookahead at '}' / end of file.
func (p *parser) endOfStatement() {
	switch {
	case p.at(token.NEWLINE):
		p.skipNewlines()
	case p.at(token.RBRACE), p.at(token.EOF):
	default:
		p.fail("expected newline after statement, found %s", p.describeCur())
	}
}

func (p *parser) parseFuncDecl() *ast.FuncDecl {
	fn := &ast.FuncDecl{StmtBase: ast.StmtBase{P: p.cur().Pos}}
	p.expect(token.FUNC)
	name := p.expect(token.IDENT)
	fn.Name, fn.NamePos = name.Lit, name.Pos

	p.expect(token.LPAREN)
	for !p.at(token.RPAREN) {
		pname := p.expect(token.IDENT)
		p.expect(token.COLON)
		ptype := p.parseType()
		fn.Params = append(fn.Params, ast.Param{Name: pname.Lit, NamePos: pname.Pos, Type: ptype})
		if !p.at(token.COMMA) {
			break
		}
		p.advance()
	}
	p.expect(token.RPAREN)

	if p.at(token.ARROW) {
		p.advance()
		fn.Ret = p.parseType()
	}
	fn.Body = p.parseBlock()
	return fn
}

func (p *parser) parseBlock() *ast.Block {
	blk := &ast.Block{StmtBase: ast.StmtBase{P: p.cur().Pos}}
	p.expect(token.LBRACE)
	p.skipNewlines()
	for !p.at(token.RBRACE) {
		if p.at(token.EOF) {
			p.fail("expected '}', found end of file")
		}
		blk.Stmts = append(blk.Stmts, p.parseStatement(false))
		p.endOfStatement()
	}
	p.expect(token.RBRACE)
	return blk
}

// ----------------------------------------------------------------- statements

func (p *parser) parseStatement(topLevel bool) ast.Stmt {
	switch p.cur().Kind {
	case token.LET, token.VAR:
		return p.parseDecl()
	case token.IF:
		return p.parseIf()
	case token.WHILE:
		return p.parseWhile()
	case token.FOR:
		return p.parseFor()
	case token.RETURN:
		return p.parseReturn()
	case token.BREAK:
		t := p.advance()
		return &ast.BreakStmt{StmtBase: ast.StmtBase{P: t.Pos}}
	case token.CONTINUE:
		t := p.advance()
		return &ast.ContinueStmt{StmtBase: ast.StmtBase{P: t.Pos}}
	case token.FUNC:
		p.fail("functions can only be declared at the top level")
	case token.RBRACE:
		p.fail("expected statement, found '}'")
	}
	return p.parseSimpleStatement()
}

func (p *parser) parseDecl() *ast.DeclStmt {
	kw := p.advance() // let or var
	d := &ast.DeclStmt{
		StmtBase: ast.StmtBase{P: kw.Pos},
		Mutable:  kw.Kind == token.VAR,
	}
	name := p.expect(token.IDENT)
	d.Name, d.NamePos = name.Lit, name.Pos
	if p.at(token.COLON) {
		p.advance()
		d.Ann = p.parseType()
	}
	if !p.at(token.ASSIGN) {
		p.fail("expected '=' (every '%s' declaration needs an initial value), found %s", kw.Lit, p.describeCur())
	}
	p.advance()
	d.Value = p.parseExpr()
	return d
}

func (p *parser) parseIf() *ast.IfStmt {
	s := &ast.IfStmt{StmtBase: ast.StmtBase{P: p.cur().Pos}}
	p.expect(token.IF)
	s.Cond = p.parseExpr()
	s.Then = p.parseBlock()

	// 'else' may follow on the same line or after newlines. If it does not,
	// rewind so the newline still terminates this statement.
	mark := p.i
	p.skipNewlines()
	if !p.at(token.ELSE) {
		p.i = mark
		return s
	}
	p.advance() // else
	if p.at(token.IF) {
		s.Else = p.parseIf()
	} else {
		s.Else = p.parseBlock()
	}
	return s
}

func (p *parser) parseWhile() *ast.WhileStmt {
	s := &ast.WhileStmt{StmtBase: ast.StmtBase{P: p.cur().Pos}}
	p.expect(token.WHILE)
	s.Cond = p.parseExpr()
	s.Body = p.parseBlock()
	return s
}

func (p *parser) parseFor() *ast.ForStmt {
	s := &ast.ForStmt{StmtBase: ast.StmtBase{P: p.cur().Pos}}
	p.expect(token.FOR)
	v := p.expect(token.IDENT)
	s.Var, s.VarPos = v.Lit, v.Pos
	p.expect(token.IN)
	s.Iter = p.parseExpr()
	s.Body = p.parseBlock()
	return s
}

func (p *parser) parseReturn() *ast.ReturnStmt {
	s := &ast.ReturnStmt{StmtBase: ast.StmtBase{P: p.cur().Pos}}
	p.expect(token.RETURN)
	if !p.at(token.NEWLINE) && !p.at(token.RBRACE) && !p.at(token.EOF) {
		s.Value = p.parseExpr()
	}
	return s
}

var assignOps = map[token.Kind]bool{
	token.ASSIGN: true, token.PLUS_ASSIGN: true, token.MINUS_ASSIGN: true,
	token.STAR_ASSIGN: true, token.SLASH_ASSIGN: true, token.PERCENT_ASSIGN: true,
}

// parseSimpleStatement parses either an assignment or an expression statement.
func (p *parser) parseSimpleStatement() ast.Stmt {
	start := p.cur().Pos
	x := p.parseExpr()
	if op := p.cur(); assignOps[op.Kind] {
		switch x.(type) {
		case *ast.Ident, *ast.IndexExpr:
		default:
			panic(token.Errorf(x.Pos(), "invalid assignment target (must be a variable or an index expression)"))
		}
		p.advance()
		return &ast.AssignStmt{
			StmtBase: ast.StmtBase{P: start},
			Target:   x,
			Op:       op.Kind,
			OpPos:    op.Pos,
			Value:    p.parseExpr(),
		}
	}
	return &ast.ExprStmt{StmtBase: ast.StmtBase{P: start}, X: x}
}

// ---------------------------------------------------------------------- types

func (p *parser) parseType() ast.TypeExpr {
	switch p.cur().Kind {
	case token.IDENT:
		t := p.advance()
		return &ast.NamedType{P: t.Pos, Name: t.Lit}
	case token.LBRACKET:
		open := p.advance()
		elem := p.parseType()
		p.expect(token.RBRACKET)
		return &ast.ListType{P: open.Pos, Elem: elem}
	}
	p.fail("expected a type, found %s", p.describeCur())
	return nil
}

// ---------------------------------------------------------------- expressions

func (p *parser) parseExpr() ast.Expr { return p.parseOr() }

func (p *parser) parseOr() ast.Expr {
	x := p.parseAnd()
	for p.at(token.OR) {
		op := p.advance()
		y := p.parseAnd()
		x = &ast.BinaryExpr{ExprBase: ast.At(x.Pos()), Op: op.Kind, OpPos: op.Pos, X: x, Y: y}
	}
	return x
}

func (p *parser) parseAnd() ast.Expr {
	x := p.parseNot()
	for p.at(token.AND) {
		op := p.advance()
		y := p.parseNot()
		x = &ast.BinaryExpr{ExprBase: ast.At(x.Pos()), Op: op.Kind, OpPos: op.Pos, X: x, Y: y}
	}
	return x
}

func (p *parser) parseNot() ast.Expr {
	if p.at(token.NOT) {
		op := p.advance()
		x := p.parseNot()
		return &ast.UnaryExpr{ExprBase: ast.At(op.Pos), Op: token.NOT, X: x}
	}
	return p.parseComparison()
}

var cmpOps = map[token.Kind]bool{
	token.EQ: true, token.NEQ: true, token.LT: true,
	token.LE: true, token.GT: true, token.GE: true,
}

func (p *parser) parseComparison() ast.Expr {
	x := p.parseAdditive()
	for cmpOps[p.cur().Kind] {
		op := p.advance()
		y := p.parseAdditive()
		x = &ast.BinaryExpr{ExprBase: ast.At(x.Pos()), Op: op.Kind, OpPos: op.Pos, X: x, Y: y}
	}
	return x
}

func (p *parser) parseAdditive() ast.Expr {
	x := p.parseMultiplicative()
	for p.at(token.PLUS) || p.at(token.MINUS) {
		op := p.advance()
		y := p.parseMultiplicative()
		x = &ast.BinaryExpr{ExprBase: ast.At(x.Pos()), Op: op.Kind, OpPos: op.Pos, X: x, Y: y}
	}
	return x
}

func (p *parser) parseMultiplicative() ast.Expr {
	x := p.parseUnary()
	for p.at(token.STAR) || p.at(token.SLASH) || p.at(token.PERCENT) {
		op := p.advance()
		y := p.parseUnary()
		x = &ast.BinaryExpr{ExprBase: ast.At(x.Pos()), Op: op.Kind, OpPos: op.Pos, X: x, Y: y}
	}
	return x
}

func (p *parser) parseUnary() ast.Expr {
	if p.at(token.MINUS) {
		op := p.advance()
		x := p.parseUnary()
		return &ast.UnaryExpr{ExprBase: ast.At(op.Pos), Op: token.MINUS, X: x}
	}
	return p.parsePostfix()
}

func (p *parser) parsePostfix() ast.Expr {
	x := p.parsePrimary()
	for {
		switch p.cur().Kind {
		case token.LPAREN:
			fun, ok := x.(*ast.Ident)
			if !ok {
				p.fail("only named functions can be called")
			}
			p.advance()
			call := &ast.CallExpr{ExprBase: ast.At(fun.Pos()), Fun: fun}
			for !p.at(token.RPAREN) {
				call.Args = append(call.Args, p.parseExpr())
				if !p.at(token.COMMA) {
					break
				}
				p.advance()
			}
			p.expect(token.RPAREN)
			x = call
		case token.LBRACKET:
			p.advance()
			idx := p.parseExpr()
			p.expect(token.RBRACKET)
			x = &ast.IndexExpr{ExprBase: ast.At(x.Pos()), X: x, Index: idx}
		default:
			return x
		}
	}
}

func (p *parser) parsePrimary() ast.Expr {
	t := p.cur()
	switch t.Kind {
	case token.INT:
		p.advance()
		return &ast.IntLit{ExprBase: ast.At(t.Pos), Raw: t.Lit}
	case token.FLOAT:
		p.advance()
		return &ast.FloatLit{ExprBase: ast.At(t.Pos), Raw: t.Lit}
	case token.STRING:
		p.advance()
		return &ast.StringLit{ExprBase: ast.At(t.Pos), Value: t.Lit}
	case token.TRUE, token.FALSE:
		p.advance()
		return &ast.BoolLit{ExprBase: ast.At(t.Pos), Value: t.Kind == token.TRUE}
	case token.IDENT:
		p.advance()
		return &ast.Ident{ExprBase: ast.At(t.Pos), Name: t.Lit}
	case token.LBRACKET:
		p.advance()
		lit := &ast.ListLit{ExprBase: ast.At(t.Pos)}
		for !p.at(token.RBRACKET) {
			lit.Elems = append(lit.Elems, p.parseExpr())
			if !p.at(token.COMMA) {
				break
			}
			p.advance()
		}
		p.expect(token.RBRACKET)
		return lit
	case token.LPAREN:
		p.advance()
		x := p.parseExpr()
		p.expect(token.RPAREN)
		return x
	}
	p.fail("expected an expression, found %s", p.describeCur())
	return nil
}
