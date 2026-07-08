package emit

import (
	"go/ast"
	"go/token"

	"github.com/tamnd/unagi/pkg/types"
)

// This file is the representation table of doc 04, the map from a proven lattice
// type to the Go type the static tier lowers it to. It is the gate every static
// value passes: a type with a representation lowers unboxed, a type without one
// keeps its boxed *objects.Object form. The scalar set landing here is int,
// float, bool, str, and a list of one of those; the aggregate and class cases
// arrive in later slices.

// Scalar names the arithmetic class of a representation, the fact the operator
// lowering branches on. NotScalar covers aggregates like a slice, which carry
// values but are not themselves arithmetic operands.
type Scalar uint8

const (
	// NotScalar is an aggregate or reference representation, not an arithmetic operand.
	NotScalar Scalar = iota
	// SBool is Go bool.
	SBool
	// SInt is Go int64, the one representation with a narrowing (overflow) guard.
	SInt
	// SFloat is Go float64.
	SFloat
	// SStr is Go string, read-only at this tier.
	SStr
)

// String names the scalar class for diagnostics.
func (s Scalar) String() string {
	switch s {
	case NotScalar:
		return "aggregate"
	case SBool:
		return "bool"
	case SInt:
		return "int"
	case SFloat:
		return "float"
	case SStr:
		return "str"
	}
	return "unknown"
}

// Repr is the unboxed Go representation of a proven Python type: the Go type text
// for signatures and casts, the scalar class the arithmetic lowering branches on,
// whether every operation on it is total (float, str, bool have no narrowing
// guard; int does not, its arithmetic guards overflow per doc 06 section 7.5),
// and, for a list, the element representation the loop and index paths need.
type Repr struct {
	Go     string
	Scalar Scalar
	Total  bool
	Elem   *Repr
}

// goType builds the go/ast type node for the representation. Scalars are a bare
// identifier; a list is a slice of its element node. The strings are compiler
// constants, never user input.
func (r Repr) goType() ast.Expr {
	if r.Elem != nil {
		return &ast.ArrayType{Elt: r.Elem.goType()}
	}
	return ident(r.Go)
}

// zero is the representation's Go zero value, the first result of an error-path
// return where the static tier bails before it has a real value.
func (r Repr) zero() ast.Expr {
	switch r.Scalar {
	case SFloat:
		return floatLit(0)
	case SInt:
		return intLit(0)
	case SBool:
		return ident("false")
	case SStr:
		return &ast.BasicLit{Kind: token.STRING, Value: `""`}
	}
	return ident("nil")
}

// Of maps a proven lattice type to its unboxed representation, reporting false
// when the type has no static form and must stay boxed. It reads only the type's
// kind and, for a list, its element, so an unproven or aggregate-of-aggregate
// type falls through to the boxed tier rather than lowering to a wrong shape.
func Of(t *types.Type) (Repr, bool) {
	switch t.Kind() {
	case types.KindBool:
		return Repr{Go: "bool", Scalar: SBool, Total: true}, true
	case types.KindInt:
		return Repr{Go: "int64", Scalar: SInt, Total: false}, true
	case types.KindFloat:
		return Repr{Go: "float64", Scalar: SFloat, Total: true}, true
	case types.KindStr:
		return Repr{Go: "string", Scalar: SStr, Total: true}, true
	case types.KindList:
		elems := t.Elems()
		if len(elems) != 1 {
			return Repr{}, false
		}
		er, ok := Of(elems[0])
		if !ok || er.Scalar == NotScalar {
			return Repr{}, false
		}
		return Repr{Go: "[]" + er.Go, Scalar: NotScalar, Total: true, Elem: &er}, true
	}
	return Repr{}, false
}
