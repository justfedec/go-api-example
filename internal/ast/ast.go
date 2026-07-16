// Package ast defines the abstract syntax tree of Inkdown programs.
//
// Expression nodes carry a mutable type slot that the checker fills in and
// the code generator reads. Positions always refer to the extracted
// (tangled) source; the driver maps them back to the Markdown document.
package ast

import (
	"github.com/justfedec/inkdown/internal/token"
	"github.com/justfedec/inkdown/internal/types"
)

// Node is implemented by every AST node.
type Node interface {
	Pos() token.Pos
}

// ---------------------------------------------------------------- Expressions

// Expr is implemented by all expression nodes.
type Expr interface {
	Node
	// Type returns the type resolved by the checker (nil before checking,
	// and nil for calls to functions that return nothing).
	Type() types.Type
	SetType(types.Type)
	exprNode()
}

// ExprBase supplies position and type storage for expression nodes.
type ExprBase struct {
	P token.Pos
	T types.Type
}

// At builds an ExprBase for a node starting at pos.
func At(pos token.Pos) ExprBase { return ExprBase{P: pos} }

func (b *ExprBase) Pos() token.Pos       { return b.P }
func (b *ExprBase) Type() types.Type     { return b.T }
func (b *ExprBase) SetType(t types.Type) { b.T = t }
func (*ExprBase) exprNode()              {}

// Ident is a reference to a variable (or, in call position, a function).
type Ident struct {
	ExprBase
	Name string
}

// IntLit is an integer literal; Raw is its decimal spelling.
type IntLit struct {
	ExprBase
	Raw string
}

// FloatLit is a float literal; Raw is its source spelling.
type FloatLit struct {
	ExprBase
	Raw string
}

// StringLit is a string literal; Value is the decoded string.
type StringLit struct {
	ExprBase
	Value string
}

// BoolLit is true or false.
type BoolLit struct {
	ExprBase
	Value bool
}

// ListLit is [e1, e2, ...].
type ListLit struct {
	ExprBase
	Elems []Expr
}

// CallExpr is a call to a named function, builtin, or record constructor;
// functions are not values, so the callee is always an identifier (dotted
// for namespaced builtins). ArgNames is empty for positional calls; record
// constructors fill it in parallel with Args (all-or-nothing).
type CallExpr struct {
	ExprBase
	Fun      *Ident
	Args     []Expr
	ArgNames []string
}

// IndexExpr is xs[i].
type IndexExpr struct {
	ExprBase
	X     Expr
	Index Expr
}

// SelectorExpr is x.field — record field access. Namespaced builtin calls
// never produce this node (they fold into the callee Ident's name).
type SelectorExpr struct {
	ExprBase
	X       Expr
	Name    string
	NamePos token.Pos
}

// UnaryExpr is -x or not x.
type UnaryExpr struct {
	ExprBase
	Op token.Kind // MINUS or NOT
	X  Expr
}

// BinaryExpr is x op y.
type BinaryExpr struct {
	ExprBase
	Op    token.Kind
	OpPos token.Pos
	X, Y  Expr
}

// ---------------------------------------------------------------- Type syntax

// TypeExpr is the syntax of a type annotation.
type TypeExpr interface {
	Node
	typeExprNode()
	String() string
}

// NamedType is a type spelled as an identifier: int, float, string, bool.
type NamedType struct {
	P    token.Pos
	Name string
}

func (t *NamedType) Pos() token.Pos { return t.P }
func (*NamedType) typeExprNode()    {}
func (t *NamedType) String() string { return t.Name }

// ListType is [T].
type ListType struct {
	P    token.Pos
	Elem TypeExpr
}

func (t *ListType) Pos() token.Pos { return t.P }
func (*ListType) typeExprNode()    {}
func (t *ListType) String() string { return "[" + t.Elem.String() + "]" }

// ----------------------------------------------------------------- Statements

// Stmt is implemented by all statement nodes.
type Stmt interface {
	Node
	stmtNode()
}

// StmtBase supplies position storage for statement nodes.
type StmtBase struct {
	P token.Pos
}

func (b *StmtBase) Pos() token.Pos { return b.P }
func (*StmtBase) stmtNode()        {}

// Block is { stmts }.
type Block struct {
	StmtBase
	Stmts []Stmt
}

// DeclStmt is a let or var declaration. The two-name form
// `let x, err = f(...)` binds a fallible builtin's error message (a string,
// "" on success) to ErrName.
type DeclStmt struct {
	StmtBase
	Name    string
	NamePos token.Pos
	ErrName string // "" for the one-name form
	ErrPos  token.Pos
	Mutable bool     // var (true) or let (false)
	Ann     TypeExpr // optional annotation, nil when inferred
	Value   Expr

	// Filled in by the checker for the code generator.
	Global    bool       // declared directly at the top level
	VarT      types.Type // resolved variable type
	Unused    bool       // local never read (codegen emits "_ = x")
	ErrUnused bool       // err binding never read
}

// AssignStmt is target = value, or a compound assignment.
type AssignStmt struct {
	StmtBase
	Target Expr       // *Ident, *IndexExpr, or *SelectorExpr
	Op     token.Kind // ASSIGN, PLUS_ASSIGN, ...
	OpPos  token.Pos
	Value  Expr
}

// IfStmt is if/else if/else. Else is nil, a *Block, or a nested *IfStmt.
type IfStmt struct {
	StmtBase
	Cond Expr
	Then *Block
	Else Stmt
}

// WhileStmt is while cond { }.
type WhileStmt struct {
	StmtBase
	Cond Expr
	Body *Block
}

// ForStmt is for x in list { }.
type ForStmt struct {
	StmtBase
	Var    string
	VarPos token.Pos
	Iter   Expr
	Body   *Block

	Unused bool // loop variable never read
}

// ReturnStmt is return [expr].
type ReturnStmt struct {
	StmtBase
	Value Expr // nil for a bare return
}

// BreakStmt is break.
type BreakStmt struct{ StmtBase }

// ContinueStmt is continue.
type ContinueStmt struct{ StmtBase }

// ExprStmt is an expression in statement position (a call, per the checker).
type ExprStmt struct {
	StmtBase
	X Expr
}

// Param is one function parameter.
type Param struct {
	Name    string
	NamePos token.Pos
	Type    TypeExpr
}

// Field is one record field.
type Field struct {
	Name    string
	NamePos token.Pos
	Type    TypeExpr
}

// RecordDecl is a top-level record type declaration.
type RecordDecl struct {
	StmtBase
	Name    string
	NamePos token.Pos
	Fields  []Field
}

// FuncDecl is a top-level function declaration.
type FuncDecl struct {
	StmtBase
	Name    string
	NamePos token.Pos
	Params  []Param
	Ret     TypeExpr // nil when the function returns nothing
	Body    *Block
}

// Program is a whole tangled program: function declarations and top-level
// statements, in source order.
type Program struct {
	Stmts []Stmt
}

func (p *Program) Pos() token.Pos {
	if len(p.Stmts) > 0 {
		return p.Stmts[0].Pos()
	}
	return token.Pos{Line: 1, Col: 1}
}
