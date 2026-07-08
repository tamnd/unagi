package emit

import (
	"go/ast"
	"go/token"
)

// This file lowers a static-to-static call: a call from one static unit to
// another, which doc 06 section 5.5 emits as a direct, monomorphic Go call the Go
// compiler can inline. There is no boxing thunk and no dispatch; the callee's Go
// signature is (args..., ) (T, error), so the call binds a value and an error,
// checks the error the way the closest example in section 11.3 does, and yields
// the value. The error check returns the caller's zero value and the propagated
// error, the D14 channel both tiers share.

// Call is a direct call to another static function. Name is the callee's Go name,
// Args are the argument expressions in order, and Ret is the callee's result
// representation. Every static function is fallible in the D14 shape, so the call
// always threads an error check.
type Call struct {
	Name string
	Args []Expr
	Ret  Repr
}

func (Call) isExpr() {}

// lowerCall lowers a static call. It lowers each argument, emits the call binding
// a value and an error temporary, appends the error check to the pending guard
// list so it sits at the enclosing statement boundary, and returns the value
// temporary as the call's result.
func (b *Builder) lowerCall(n Call) (ast.Expr, Repr, error) {
	args := make([]ast.Expr, len(n.Args))
	for i, a := range n.Args {
		x, _, err := b.lowerExpr(a)
		if err != nil {
			return nil, Repr{}, err
		}
		args[i] = x
	}
	val, exc := b.temp(), b.errName()
	b.pre = append(b.pre,
		&ast.AssignStmt{
			Lhs: []ast.Expr{ident(val), ident(exc)},
			Tok: token.DEFINE,
			Rhs: []ast.Expr{callExpr(ident(n.Name), args...)},
		},
		ifStmt(binary(token.NEQ, ident(exc), ident("nil")),
			ret(b.ret.zero(), ident(exc))),
	)
	return ident(val), n.Ret, nil
}
