// Package check performs semantic analysis: name resolution, type checking
// and inference, mutability and control-flow rules. It annotates the AST
// with resolved types for the code generator and stops at the first error.
package check

import (
	"fmt"

	"github.com/justfedec/inkdown/internal/ast"
	"github.com/justfedec/inkdown/internal/token"
	"github.com/justfedec/inkdown/internal/types"
)

// Check analyzes prog in three passes: collect function signatures, check
// top-level statements in execution order, then check function bodies with
// every global in scope.
func Check(prog *ast.Program) (err error) {
	defer func() {
		if r := recover(); r != nil {
			if cerr, ok := r.(*token.Error); ok {
				err = cerr
				return
			}
			panic(r)
		}
	}()

	c := &checker{global: newScope(nil)}

	// Pass 1: function signatures, so calls work regardless of order.
	for _, s := range prog.Stmts {
		if fn, ok := s.(*ast.FuncDecl); ok {
			c.declareFunc(fn)
		}
	}

	// Pass 2: top-level statements, in the order they will execute.
	for _, s := range prog.Stmts {
		if _, ok := s.(*ast.FuncDecl); ok {
			continue
		}
		c.stmt(c.global, s)
	}

	// Pass 3: function bodies.
	for _, s := range prog.Stmts {
		if fn, ok := s.(*ast.FuncDecl); ok {
			c.funcBody(fn)
		}
	}
	return nil
}

// ------------------------------------------------------------------- symbols

type symKind int

const (
	symLet symKind = iota
	symVar
	symParam
	symLoopVar
	symFunc
)

type symbol struct {
	name string
	kind symKind
	typ  types.Type // variables only
	sig  *sig       // functions only
	used bool

	decl    *ast.DeclStmt // local declarations, for unused marking
	forStmt *ast.ForStmt  // loop variables, for unused marking
}

type sig struct {
	params []types.Type
	ret    types.Type // nil: no return value
}

type scope struct {
	parent *scope
	syms   map[string]*symbol
	order  []*symbol
}

func newScope(parent *scope) *scope {
	return &scope{parent: parent, syms: map[string]*symbol{}}
}

func (s *scope) lookup(name string) *symbol {
	for sc := s; sc != nil; sc = sc.parent {
		if sym, ok := sc.syms[name]; ok {
			return sym
		}
	}
	return nil
}

// builtinNames are predeclared and cannot be redeclared or used as plain
// values. The callable ones are dispatched by name in call(); "string" and
// "bool" are reserved because they name types.
var builtinNames = map[string]bool{
	"print": true, "len": true, "range": true, "push": true,
	"str": true, "int": true, "float": true, "string": true, "bool": true,
}

func (c *checker) declare(sc *scope, pos token.Pos, sym *symbol) {
	if builtinNames[sym.name] {
		c.errf(pos, "cannot redeclare builtin '%s'", sym.name)
	}
	if _, exists := sc.syms[sym.name]; exists {
		c.errf(pos, "'%s' is already declared in this scope", sym.name)
	}
	sc.syms[sym.name] = sym
	sc.order = append(sc.order, sym)
}

// closeScope marks declarations whose local was never read, so the code
// generator can keep the emitted Go compilable ("declared and not used").
func closeScope(sc *scope) {
	for _, sym := range sc.order {
		if sym.used {
			continue
		}
		if sym.decl != nil && !sym.decl.Global {
			sym.decl.Unused = true
		}
		if sym.forStmt != nil {
			sym.forStmt.Unused = true
		}
	}
}

// ------------------------------------------------------------------- checker

type checker struct {
	global    *scope
	fn        *ast.FuncDecl // current function, nil at the top level
	fnSig     *sig
	loopDepth int
}

func (c *checker) errf(pos token.Pos, format string, args ...any) {
	panic(token.Errorf(pos, format, args...))
}

func (c *checker) declareFunc(fn *ast.FuncDecl) {
	fsig := &sig{}
	for _, p := range fn.Params {
		fsig.params = append(fsig.params, c.resolveType(p.Type))
	}
	if fn.Ret != nil {
		fsig.ret = c.resolveType(fn.Ret)
	}
	c.declare(c.global, fn.NamePos, &symbol{name: fn.Name, kind: symFunc, sig: fsig})
}

func (c *checker) funcBody(fn *ast.FuncDecl) {
	sym := c.global.lookup(fn.Name)
	c.fn, c.fnSig = fn, sym.sig
	defer func() { c.fn, c.fnSig = nil, nil }()

	sc := newScope(c.global)
	for i, p := range fn.Params {
		c.declare(sc, p.NamePos, &symbol{name: p.Name, kind: symParam, typ: sym.sig.params[i]})
	}
	c.stmts(sc, fn.Body.Stmts)
	// Marks unused body locals for codegen. Parameters may stay unused (as
	// in Go); they carry no decl node, so closeScope never marks them.
	closeScope(sc)

	if sym.sig.ret != nil && !blockTerminates(fn.Body) {
		c.errf(fn.NamePos, "missing return: function '%s' returns %s but its body can finish without a return statement", fn.Name, sym.sig.ret)
	}
}

// ------------------------------------------------------------------- types

func (c *checker) resolveType(t ast.TypeExpr) types.Type {
	switch t := t.(type) {
	case *ast.NamedType:
		switch t.Name {
		case "int":
			return types.Int
		case "float":
			return types.Float
		case "string":
			return types.String
		case "bool":
			return types.Bool
		case "str":
			c.errf(t.Pos(), "unknown type 'str' (the type is called 'string')")
		}
		c.errf(t.Pos(), "unknown type '%s'", t.Name)
	case *ast.ListType:
		return &types.List{Elem: c.resolveType(t.Elem)}
	}
	panic("unreachable")
}

// ---------------------------------------------------------------- statements

func (c *checker) stmts(sc *scope, list []ast.Stmt) {
	for _, s := range list {
		c.stmt(sc, s)
	}
}

func (c *checker) block(parent *scope, b *ast.Block) {
	sc := newScope(parent)
	c.stmts(sc, b.Stmts)
	closeScope(sc)
}

func (c *checker) stmt(sc *scope, s ast.Stmt) {
	switch s := s.(type) {
	case *ast.DeclStmt:
		c.declStmt(sc, s)
	case *ast.AssignStmt:
		c.assignStmt(sc, s)
	case *ast.IfStmt:
		c.condition(sc, s.Cond, "if")
		c.block(sc, s.Then)
		if s.Else != nil {
			switch e := s.Else.(type) {
			case *ast.Block:
				c.block(sc, e)
			case *ast.IfStmt:
				c.stmt(sc, e)
			}
		}
	case *ast.WhileStmt:
		c.condition(sc, s.Cond, "while")
		c.loopBody(sc, s.Body, nil)
	case *ast.ForStmt:
		iterT := c.exprValue(sc, s.Iter, nil)
		list, ok := iterT.(*types.List)
		if !ok {
			c.errf(s.Iter.Pos(), "'for' needs a list to iterate over, got %s", iterT)
		}
		c.loopBody(sc, s.Body, &symbol{name: s.Var, kind: symLoopVar, typ: list.Elem, forStmt: s})
	case *ast.ReturnStmt:
		c.returnStmt(sc, s)
	case *ast.BreakStmt:
		if c.loopDepth == 0 {
			c.errf(s.Pos(), "'break' is only allowed inside a loop")
		}
	case *ast.ContinueStmt:
		if c.loopDepth == 0 {
			c.errf(s.Pos(), "'continue' is only allowed inside a loop")
		}
	case *ast.ExprStmt:
		call, ok := s.X.(*ast.CallExpr)
		if !ok {
			c.errf(s.X.Pos(), "only calls can be used as statements")
		}
		t := c.expr(sc, call, nil)
		// User functions may have their result discarded (as in Go);
		// value-only builtins may not.
		if t != nil && builtinNames[call.Fun.Name] {
			c.errf(call.Pos(), "result of %s(...) is unused", call.Fun.Name)
		}
	case *ast.FuncDecl:
		// Handled by Check's passes; the parser only allows them at the top level.
	default:
		panic(fmt.Sprintf("unhandled statement %T", s))
	}
}

func (c *checker) loopBody(parent *scope, body *ast.Block, loopVar *symbol) {
	sc := newScope(parent)
	if loopVar != nil {
		c.declare(sc, loopVar.forStmt.VarPos, loopVar)
	}
	c.loopDepth++
	c.stmts(sc, body.Stmts)
	c.loopDepth--
	closeScope(sc)
}

func (c *checker) condition(sc *scope, e ast.Expr, kw string) {
	t := c.exprValue(sc, e, types.Bool)
	if !t.Equal(types.Bool) {
		c.errf(e.Pos(), "'%s' condition must be bool, got %s", kw, t)
	}
}

func (c *checker) declStmt(sc *scope, s *ast.DeclStmt) {
	var expected types.Type
	if s.Ann != nil {
		expected = c.resolveType(s.Ann)
	}
	got := c.exprValue(sc, s.Value, expected)
	if expected != nil && !got.Equal(expected) {
		c.errf(s.Value.Pos(), "cannot initialize '%s' (declared %s) with a %s value", s.Name, expected, got)
	}
	if expected == nil {
		expected = got
	}

	s.Global = sc == c.global
	s.VarT = expected

	kind := symLet
	if s.Mutable {
		kind = symVar
	}
	c.declare(sc, s.NamePos, &symbol{name: s.Name, kind: kind, typ: expected, decl: s})
}

func (c *checker) assignStmt(sc *scope, s *ast.AssignStmt) {
	var targetT types.Type
	switch target := s.Target.(type) {
	case *ast.Ident:
		sym := c.resolveVar(sc, target)
		c.requireMutable(s.Pos(), sym, "reassigned")
		if s.Op != token.ASSIGN {
			sym.used = true // compound assignment reads the old value
		}
		targetT = sym.typ
		target.SetType(sym.typ)
	case *ast.IndexExpr:
		root := rootIdent(target)
		if root == nil {
			c.errf(target.Pos(), "invalid assignment target")
		}
		sym := c.resolveVar(sc, root)
		c.requireMutable(s.Pos(), sym, "modified")
		targetT = c.exprValue(sc, target, nil)
	default:
		c.errf(s.Target.Pos(), "invalid assignment target")
	}

	if s.Op == token.ASSIGN {
		got := c.exprValue(sc, s.Value, targetT)
		if !got.Equal(targetT) {
			c.errf(s.Value.Pos(), "cannot assign a %s value to a target of type %s", got, targetT)
		}
		return
	}
	// Compound assignment: same typing as the underlying binary operator.
	binOp := map[token.Kind]token.Kind{
		token.PLUS_ASSIGN: token.PLUS, token.MINUS_ASSIGN: token.MINUS,
		token.STAR_ASSIGN: token.STAR, token.SLASH_ASSIGN: token.SLASH,
		token.PERCENT_ASSIGN: token.PERCENT,
	}[s.Op]
	got := c.exprValue(sc, s.Value, targetT)
	res := c.binaryType(s.OpPos, binOp, targetT, got)
	if !res.Equal(targetT) {
		c.errf(s.OpPos, "operator %s changes the type of the target", s.Op)
	}
}

func (c *checker) resolveVar(sc *scope, id *ast.Ident) *symbol {
	sym := sc.lookup(id.Name)
	if sym == nil {
		if builtinNames[id.Name] {
			c.errf(id.Pos(), "cannot assign to builtin '%s'", id.Name)
		}
		c.errf(id.Pos(), "'%s' is not defined", id.Name)
	}
	if sym.kind == symFunc {
		c.errf(id.Pos(), "cannot assign to function '%s'", id.Name)
	}
	return sym
}

func (c *checker) requireMutable(pos token.Pos, sym *symbol, action string) {
	switch sym.kind {
	case symVar:
	case symLet:
		c.errf(pos, "'%s' was declared with 'let' and cannot be %s (declare it with 'var')", sym.name, action)
	case symParam:
		c.errf(pos, "parameter '%s' cannot be %s (parameters are immutable)", sym.name, action)
	case symLoopVar:
		c.errf(pos, "loop variable '%s' cannot be %s", sym.name, action)
	}
}

func (c *checker) returnStmt(sc *scope, s *ast.ReturnStmt) {
	if c.fn == nil {
		c.errf(s.Pos(), "'return' is not allowed at the top level")
	}
	if c.fnSig.ret == nil {
		if s.Value != nil {
			c.errf(s.Value.Pos(), "function '%s' does not declare a return type, so 'return' must be bare", c.fn.Name)
		}
		return
	}
	if s.Value == nil {
		c.errf(s.Pos(), "missing return value (function '%s' returns %s)", c.fn.Name, c.fnSig.ret)
	}
	got := c.exprValue(sc, s.Value, c.fnSig.ret)
	if !got.Equal(c.fnSig.ret) {
		c.errf(s.Value.Pos(), "function '%s' returns %s, but this returns a %s value", c.fn.Name, c.fnSig.ret, got)
	}
}

// pushTargetIndexHasCall reports whether any index expression along a push
// target (xs[i][j]) contains a call.
func pushTargetIndexHasCall(e ast.Expr) bool {
	for {
		idx, ok := e.(*ast.IndexExpr)
		if !ok {
			return false
		}
		if exprHasCall(idx.Index) {
			return true
		}
		e = idx.X
	}
}

func exprHasCall(e ast.Expr) bool {
	switch e := e.(type) {
	case *ast.CallExpr:
		return true
	case *ast.UnaryExpr:
		return exprHasCall(e.X)
	case *ast.BinaryExpr:
		return exprHasCall(e.X) || exprHasCall(e.Y)
	case *ast.IndexExpr:
		return exprHasCall(e.X) || exprHasCall(e.Index)
	case *ast.ListLit:
		for _, el := range e.Elems {
			if exprHasCall(el) {
				return true
			}
		}
	}
	return false
}

// rootIdent unwraps xs[i][j]... to the base identifier, or nil.
func rootIdent(e ast.Expr) *ast.Ident {
	for {
		switch x := e.(type) {
		case *ast.Ident:
			return x
		case *ast.IndexExpr:
			e = x.X
		default:
			return nil
		}
	}
}

// blockTerminates reports whether a block certainly ends in a return.
func blockTerminates(b *ast.Block) bool {
	if len(b.Stmts) == 0 {
		return false
	}
	return stmtTerminates(b.Stmts[len(b.Stmts)-1])
}

func stmtTerminates(s ast.Stmt) bool {
	switch s := s.(type) {
	case *ast.ReturnStmt:
		return true
	case *ast.IfStmt:
		if s.Else == nil || !blockTerminates(s.Then) {
			return false
		}
		switch e := s.Else.(type) {
		case *ast.Block:
			return blockTerminates(e)
		case *ast.IfStmt:
			return stmtTerminates(e)
		}
	}
	return false
}

// --------------------------------------------------------------- expressions

// exprValue checks e and requires it to produce a value.
func (c *checker) exprValue(sc *scope, e ast.Expr, expected types.Type) types.Type {
	t := c.expr(sc, e, expected)
	if t == nil {
		call := e.(*ast.CallExpr)
		c.errf(e.Pos(), "%s(...) does not return a value, so it cannot be used here", call.Fun.Name)
	}
	return t
}

// expr checks e, annotates it with its type, and returns that type (nil for
// calls to functions with no return value). expected guides empty list
// literals only.
func (c *checker) expr(sc *scope, e ast.Expr, expected types.Type) types.Type {
	t := c.exprInner(sc, e, expected)
	e.SetType(t)
	return t
}

func (c *checker) exprInner(sc *scope, e ast.Expr, expected types.Type) types.Type {
	switch e := e.(type) {
	case *ast.IntLit:
		return types.Int
	case *ast.FloatLit:
		return types.Float
	case *ast.StringLit:
		return types.String
	case *ast.BoolLit:
		return types.Bool

	case *ast.Ident:
		sym := sc.lookup(e.Name)
		if sym == nil {
			if builtinNames[e.Name] {
				c.errf(e.Pos(), "builtin '%s' can only be called", e.Name)
			}
			c.errf(e.Pos(), "'%s' is not defined", e.Name)
		}
		if sym.kind == symFunc {
			c.errf(e.Pos(), "function '%s' can only be called (functions are not values in v1)", e.Name)
		}
		sym.used = true
		return sym.typ

	case *ast.ListLit:
		return c.listLit(sc, e, expected)

	case *ast.CallExpr:
		return c.call(sc, e, expected)

	case *ast.IndexExpr:
		xt := c.exprValue(sc, e.X, nil)
		list, ok := xt.(*types.List)
		if !ok {
			if xt.Equal(types.String) {
				c.errf(e.X.Pos(), "strings cannot be indexed in v1")
			}
			c.errf(e.X.Pos(), "cannot index a value of type %s", xt)
		}
		it := c.exprValue(sc, e.Index, types.Int)
		if !it.Equal(types.Int) {
			c.errf(e.Index.Pos(), "list index must be int, got %s", it)
		}
		return list.Elem

	case *ast.UnaryExpr:
		xt := c.exprValue(sc, e.X, nil)
		if e.Op == token.NOT {
			if !xt.Equal(types.Bool) {
				c.errf(e.X.Pos(), "'not' needs a bool operand, got %s", xt)
			}
			return types.Bool
		}
		if !types.IsNumeric(xt) {
			c.errf(e.X.Pos(), "unary '-' needs an int or float operand, got %s", xt)
		}
		return xt

	case *ast.BinaryExpr:
		xt := c.exprValue(sc, e.X, nil)
		yt := c.exprValue(sc, e.Y, xt)
		return c.binaryType(e.OpPos, e.Op, xt, yt)
	}
	panic(fmt.Sprintf("unhandled expression %T", e))
}

// ---------------------------------------------------------------------- calls

func (c *checker) call(sc *scope, e *ast.CallExpr, _ types.Type) types.Type {
	name := e.Fun.Name

	argCount := func(n int) {
		if len(e.Args) != n {
			plural := "s"
			if n == 1 {
				plural = ""
			}
			c.errf(e.Pos(), "%s() takes %d argument%s, got %d", name, n, plural, len(e.Args))
		}
	}

	switch name {
	case "print":
		for _, a := range e.Args {
			c.exprValue(sc, a, nil)
		}
		return nil

	case "len":
		argCount(1)
		t := c.exprValue(sc, e.Args[0], nil)
		if _, isList := t.(*types.List); !isList && !t.Equal(types.String) {
			c.errf(e.Args[0].Pos(), "len() needs a string or a list, got %s", t)
		}
		return types.Int

	case "range":
		argCount(2)
		for _, a := range e.Args {
			if t := c.exprValue(sc, a, types.Int); !t.Equal(types.Int) {
				c.errf(a.Pos(), "range() bounds must be int, got %s", t)
			}
		}
		return &types.List{Elem: types.Int}

	case "push":
		argCount(2)
		target := e.Args[0]
		root := rootIdent(target)
		if root == nil {
			c.errf(target.Pos(), "the first argument of push() must be a list variable (or an index into one)")
		}
		// The generated Go spells the target twice (xs = append(xs, v)), so
		// indices with calls would run those calls twice.
		if pushTargetIndexHasCall(target) {
			c.errf(target.Pos(), "push() target indices must not contain function calls (they would be evaluated twice); store the index in a variable first")
		}
		sym := c.resolveVar(sc, root)
		c.requireMutable(target.Pos(), sym, "modified by push()")
		t := c.exprValue(sc, target, nil)
		list, ok := t.(*types.List)
		if !ok {
			c.errf(target.Pos(), "push() needs a list as its first argument, got %s", t)
		}
		vt := c.exprValue(sc, e.Args[1], list.Elem)
		if !vt.Equal(list.Elem) {
			c.errf(e.Args[1].Pos(), "cannot push a %s value into a %s", vt, list)
		}
		return nil

	case "str":
		argCount(1)
		c.exprValue(sc, e.Args[0], nil)
		return types.String

	case "int", "float":
		argCount(1)
		t := c.exprValue(sc, e.Args[0], nil)
		if !types.IsNumeric(t) && !t.Equal(types.String) {
			c.errf(e.Args[0].Pos(), "%s() takes an int, float, or string, got %s", name, t)
		}
		if name == "int" {
			return types.Int
		}
		return types.Float

	case "string", "bool":
		suggestion := ""
		if name == "string" {
			suggestion = " (use str(x) to convert to string)"
		}
		c.errf(e.Pos(), "'%s' is a type, not a function%s", name, suggestion)
	}

	sym := sc.lookup(name)
	if sym == nil {
		c.errf(e.Pos(), "'%s' is not defined", name)
	}
	if sym.kind != symFunc {
		c.errf(e.Pos(), "'%s' is a variable, not a function", name)
	}
	sym.used = true
	if len(e.Args) != len(sym.sig.params) {
		c.errf(e.Pos(), "function '%s' takes %d argument(s), got %d", name, len(sym.sig.params), len(e.Args))
	}
	for i, a := range e.Args {
		want := sym.sig.params[i]
		got := c.exprValue(sc, a, want)
		if !got.Equal(want) {
			c.errf(a.Pos(), "argument %d of '%s' must be %s, got %s", i+1, name, want, got)
		}
	}
	return sym.sig.ret
}

func (c *checker) binaryType(pos token.Pos, op token.Kind, xt, yt types.Type) types.Type {
	mismatch := func() {
		c.errf(pos, "operator %s has mismatched operand types (%s and %s)", op, xt, yt)
	}
	switch op {
	case token.AND, token.OR:
		if !xt.Equal(types.Bool) || !yt.Equal(types.Bool) {
			c.errf(pos, "operator %s needs bool operands (got %s and %s)", op, xt, yt)
		}
		return types.Bool

	case token.EQ, token.NEQ:
		if !xt.Equal(yt) {
			mismatch()
		}
		if !types.IsComparable(xt) {
			c.errf(pos, "values of type %s cannot be compared with %s", xt, op)
		}
		return types.Bool

	case token.LT, token.LE, token.GT, token.GE:
		if !xt.Equal(yt) {
			mismatch()
		}
		if !types.IsOrdered(xt) {
			c.errf(pos, "values of type %s cannot be ordered with %s", xt, op)
		}
		return types.Bool

	case token.PLUS:
		if !xt.Equal(yt) {
			mismatch()
		}
		if !types.IsNumeric(xt) && !xt.Equal(types.String) {
			c.errf(pos, "operator '+' works on int, float, or string, got %s", xt)
		}
		return xt

	case token.MINUS, token.STAR, token.SLASH:
		if !xt.Equal(yt) {
			mismatch()
		}
		if !types.IsNumeric(xt) {
			c.errf(pos, "operator %s needs int or float operands, got %s", op, xt)
		}
		return xt

	case token.PERCENT:
		if !xt.Equal(types.Int) || !yt.Equal(types.Int) {
			c.errf(pos, "operator '%%' needs int operands (got %s and %s)", xt, yt)
		}
		return types.Int
	}
	panic("unreachable")
}

func (c *checker) listLit(sc *scope, e *ast.ListLit, expected types.Type) types.Type {
	var elemExpected types.Type
	if lt, ok := expected.(*types.List); ok {
		elemExpected = lt.Elem
	}
	if len(e.Elems) == 0 {
		if elemExpected == nil {
			c.errf(e.Pos(), "cannot infer the type of an empty list here; add a type annotation like ': [int]'")
		}
		return expected
	}
	first := c.exprValue(sc, e.Elems[0], elemExpected)
	for _, el := range e.Elems[1:] {
		t := c.exprValue(sc, el, first)
		if !t.Equal(first) {
			c.errf(el.Pos(), "list elements must all have the same type (%s, then %s)", first, t)
		}
	}
	return &types.List{Elem: first}
}
