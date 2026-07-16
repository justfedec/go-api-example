package ast

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/justfedec/inkdown/internal/token"
)

// Dump renders a node as a compact s-expression. It exists for parser tests
// and debugging; the format is stable but unspecified.
func Dump(n Node) string {
	var b strings.Builder
	dump(&b, n)
	return b.String()
}

var opSpelling = map[token.Kind]string{
	token.PLUS: "+", token.MINUS: "-", token.STAR: "*", token.SLASH: "/",
	token.PERCENT: "%", token.EQ: "==", token.NEQ: "!=", token.LT: "<",
	token.LE: "<=", token.GT: ">", token.GE: ">=", token.AND: "and",
	token.OR: "or", token.NOT: "not",
	token.ASSIGN: "=", token.PLUS_ASSIGN: "+=", token.MINUS_ASSIGN: "-=",
	token.STAR_ASSIGN: "*=", token.SLASH_ASSIGN: "/=", token.PERCENT_ASSIGN: "%=",
}

func dump(b *strings.Builder, n Node) {
	switch n := n.(type) {
	case *Program:
		b.WriteString("(program")
		for _, s := range n.Stmts {
			b.WriteByte(' ')
			dump(b, s)
		}
		b.WriteByte(')')

	case *Ident:
		b.WriteString(n.Name)
	case *IntLit:
		b.WriteString(n.Raw)
	case *FloatLit:
		b.WriteString(n.Raw)
	case *StringLit:
		b.WriteString(strconv.Quote(n.Value))
	case *BoolLit:
		fmt.Fprintf(b, "%v", n.Value)
	case *ListLit:
		b.WriteString("(list")
		for _, e := range n.Elems {
			b.WriteByte(' ')
			dump(b, e)
		}
		b.WriteByte(')')
	case *CallExpr:
		b.WriteString("(call " + n.Fun.Name)
		for i, a := range n.Args {
			b.WriteByte(' ')
			if len(n.ArgNames) > 0 {
				b.WriteString(n.ArgNames[i] + ": ")
			}
			dump(b, a)
		}
		b.WriteByte(')')
	case *IndexExpr:
		b.WriteString("(index ")
		dump(b, n.X)
		b.WriteByte(' ')
		dump(b, n.Index)
		b.WriteByte(')')
	case *SelectorExpr:
		b.WriteString("(sel ")
		dump(b, n.X)
		b.WriteString(" " + n.Name + ")")
	case *UnaryExpr:
		op := opSpelling[n.Op]
		if n.Op == token.MINUS {
			op = "neg"
		}
		b.WriteString("(" + op + " ")
		dump(b, n.X)
		b.WriteByte(')')
	case *BinaryExpr:
		b.WriteString("(" + opSpelling[n.Op] + " ")
		dump(b, n.X)
		b.WriteByte(' ')
		dump(b, n.Y)
		b.WriteByte(')')

	case *NamedType:
		b.WriteString(n.String())
	case *ListType:
		b.WriteString(n.String())

	case *Block:
		b.WriteString("(block")
		for _, s := range n.Stmts {
			b.WriteByte(' ')
			dump(b, s)
		}
		b.WriteByte(')')
	case *DeclStmt:
		kw := "let"
		if n.Mutable {
			kw = "var"
		}
		b.WriteString("(" + kw + " " + n.Name)
		if n.ErrName != "" {
			b.WriteString(" " + n.ErrName)
		}
		if n.Ann != nil {
			b.WriteString(":" + n.Ann.String())
		}
		b.WriteByte(' ')
		dump(b, n.Value)
		b.WriteByte(')')
	case *AssignStmt:
		b.WriteString("(" + opSpelling[n.Op] + " ")
		dump(b, n.Target)
		b.WriteByte(' ')
		dump(b, n.Value)
		b.WriteByte(')')
	case *IfStmt:
		b.WriteString("(if ")
		dump(b, n.Cond)
		b.WriteByte(' ')
		dump(b, n.Then)
		if n.Else != nil {
			b.WriteByte(' ')
			dump(b, n.Else)
		}
		b.WriteByte(')')
	case *WhileStmt:
		b.WriteString("(while ")
		dump(b, n.Cond)
		b.WriteByte(' ')
		dump(b, n.Body)
		b.WriteByte(')')
	case *ForStmt:
		b.WriteString("(for " + n.Var + " ")
		dump(b, n.Iter)
		b.WriteByte(' ')
		dump(b, n.Body)
		b.WriteByte(')')
	case *ReturnStmt:
		if n.Value == nil {
			b.WriteString("(return)")
		} else {
			b.WriteString("(return ")
			dump(b, n.Value)
			b.WriteByte(')')
		}
	case *BreakStmt:
		b.WriteString("(break)")
	case *ContinueStmt:
		b.WriteString("(continue)")
	case *ExprStmt:
		b.WriteString("(expr ")
		dump(b, n.X)
		b.WriteByte(')')
	case *RecordDecl:
		b.WriteString("(record " + n.Name)
		for _, f := range n.Fields {
			b.WriteString(" (" + f.Name + " " + f.Type.String() + ")")
		}
		b.WriteByte(')')
	case *FuncDecl:
		b.WriteString("(func " + n.Name + " (")
		for i, p := range n.Params {
			if i > 0 {
				b.WriteByte(' ')
			}
			b.WriteString("(" + p.Name + " " + p.Type.String() + ")")
		}
		b.WriteString(")")
		if n.Ret != nil {
			b.WriteString(" -> " + n.Ret.String())
		}
		b.WriteByte(' ')
		dump(b, n.Body)
		b.WriteByte(')')

	default:
		fmt.Fprintf(b, "(?%T)", n)
	}
}
