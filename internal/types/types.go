// Package types defines Inkdown's type system: the four basic types and
// lists. It is deliberately free of dependencies so both the AST (which
// carries inferred types) and the checker can import it.
package types

// Type is the interface implemented by all Inkdown types.
type Type interface {
	String() string
	Equal(Type) bool
}

// BasicKind enumerates the non-composite types.
type BasicKind int

const (
	IntKind BasicKind = iota
	FloatKind
	StringKind
	BoolKind
)

// Basic is one of int, float, string, bool.
type Basic struct {
	Kind BasicKind
	name string
}

var (
	Int    = &Basic{IntKind, "int"}
	Float  = &Basic{FloatKind, "float"}
	String = &Basic{StringKind, "string"}
	Bool   = &Basic{BoolKind, "bool"}
)

func (b *Basic) String() string { return b.name }

func (b *Basic) Equal(other Type) bool {
	o, ok := other.(*Basic)
	return ok && o.Kind == b.Kind
}

// List is the type of [T] values.
type List struct {
	Elem Type
}

func (l *List) String() string { return "[" + l.Elem.String() + "]" }

func (l *List) Equal(other Type) bool {
	o, ok := other.(*List)
	return ok && l.Elem.Equal(o.Elem)
}

// Opaque is a nominal handle type (server, request, response). Values exist
// only through builtins: the type has no literal syntax and no operators.
// Because IsNumeric/IsOrdered/IsComparable accept only *Basic, arithmetic,
// ordering, equality, indexing, and iteration are all rejected automatically.
type Opaque struct {
	Name string
}

func (o *Opaque) String() string { return o.Name }

func (o *Opaque) Equal(other Type) bool {
	t, ok := other.(*Opaque)
	return ok && t.Name == o.Name
}

// IsNumeric reports whether t is int or float.
func IsNumeric(t Type) bool {
	b, ok := t.(*Basic)
	return ok && (b.Kind == IntKind || b.Kind == FloatKind)
}

// IsOrdered reports whether values of t can be compared with < <= > >=.
func IsOrdered(t Type) bool {
	b, ok := t.(*Basic)
	return ok && (b.Kind == IntKind || b.Kind == FloatKind || b.Kind == StringKind)
}

// IsComparable reports whether values of t can be compared with == and !=.
func IsComparable(t Type) bool {
	_, ok := t.(*Basic)
	return ok
}
