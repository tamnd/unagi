package lower

import (
	"fmt"
	"go/ast"
	"go/token"

	"github.com/tamnd/unagi/pkg/frontend"
)

// This file builds the boxed-to-static entry shim. The partitioner proves some
// functions static and the build emits their unboxed Go beside the boxed module,
// but boxed callers still hold objects.Object arguments, so something has to
// cross the boundary. The shim is that crossing: it takes the boxed arguments a
// direct positional call passes, guards each one's dynamic type against the
// representation the static form expects, unboxes and enters the static body when
// every guard holds, and reboxes the native result. A type that does not match
// falls back to the boxed form, which is always correct, so the shim can only
// make a matching call faster, never a mismatched call wrong.
//
// The guard is exact on the dynamic type, not a coercion. A float parameter
// admits only a float object, never an int, because CPython does not coerce a
// positional argument to its annotation: scale(3, 4) with float parameters runs
// the body on ints and yields an int, which the static float form would not
// reproduce. Matching the exact type name and requiring the unbox to succeed
// keeps a spilled big int, a bool where an int is wanted, and a str subclass off
// the static path, each of which the boxed form handles instead.

// entryShimDecl builds one def's entry shim. The shim signature takes the def's
// arguments as objects.Object and returns the boxed (Object, error) pair, so a
// caller that already calls the boxed form by the same arity reaches the shim
// with no change at the call site beyond the routed name.
func (e *emitter) entryShimDecl(d *frontend.FuncDef, se StaticEntry) *ast.FuncDecl {
	n := len(se.Params)
	pnames := make([]string, n)
	pfields := make([]*ast.Field, n)
	for i := range n {
		pnames[i] = fmt.Sprintf("p%d", i)
		pfields[i] = field(e.obj("Object"), pnames[i])
	}

	// fallback is the boxed form every failed guard falls back to, re-run on the
	// original boxed arguments, which is always correct. It is rebuilt fresh each
	// time so no AST node is shared between two return sites.
	fallback := func() *ast.BlockStmt {
		args := make([]ast.Expr, n)
		for i := range n {
			args[i] = ident(pnames[i])
		}
		return block(&ast.ReturnStmt{Results: []ast.Expr{
			callExpr(ident(e.defName(d.Name)), args...),
		}})
	}

	var body []ast.Stmt
	// Unbox every scalar parameter and OR its two failure terms into the guard: the
	// unbox did not succeed, or the dynamic type is not the exact one the static
	// form expects. A shape parameter cannot fold into a boolean OR, since
	// materializing it reads slots that can each fail, so it emits its own guarded
	// early returns instead. A zero-parameter function skips the guard and enters
	// directly.
	var guard ast.Expr
	for i := range n {
		p := se.Params[i]
		if p.Shape != nil {
			body = append(body, e.materializeShape(i, p.Shape, pnames[i], fallback)...)
			continue
		}
		xname := fmt.Sprintf("x%d", i)
		okname := fmt.Sprintf("ok%d", i)
		body = append(body, assign(token.DEFINE,
			[]ast.Expr{ident(xname), ident(okname)},
			callExpr(e.obj(unboxAccessor(p.Scalar)), ident(pnames[i]))))
		guard = orJoin(guard, notExpr(ident(okname)))
		guard = orJoin(guard, &ast.BinaryExpr{
			X:  callExpr(sel(pnames[i], "TypeName")),
			Op: token.NEQ,
			Y:  strLit(scalarTypeName(p.Scalar)),
		})
	}
	if guard != nil {
		body = append(body, &ast.IfStmt{Cond: guard, Body: fallback()})
	}

	// Enter the static body with the unboxed values, handle the error channel,
	// and rebox the native result. When the static form can deopt, the error may
	// be the deopt sentinel rather than a raised exception: the static form gave
	// up and its boxed twin already produced the result, so the shim returns that
	// boxed value directly instead of surfacing it as an error.
	callArgs := make([]ast.Expr, n)
	for i := range n {
		callArgs[i] = ident(fmt.Sprintf("x%d", i))
	}
	errBody := []ast.Stmt{&ast.ReturnStmt{Results: []ast.Expr{ident("nil"), ident("err")}}}
	if se.Deopt {
		errBody = append([]ast.Stmt{&ast.IfStmt{
			Init: assign(token.DEFINE,
				[]ast.Expr{ident("d"), ident("ok")},
				&ast.TypeAssertExpr{X: ident("err"), Type: &ast.StarExpr{X: e.obj("Deopt")}}),
			Cond: ident("ok"),
			Body: block(&ast.ReturnStmt{Results: []ast.Expr{sel("d", "Value"), ident("nil")}}),
		}}, errBody...)
	}
	body = append(body,
		assign(token.DEFINE, []ast.Expr{ident("r"), ident("err")},
			callExpr(ident(se.Static), callArgs...)),
		&ast.IfStmt{
			Cond: errNotNil(),
			Body: block(errBody...),
		},
		&ast.ReturnStmt{Results: []ast.Expr{
			callExpr(e.obj(reboxConstructor(se.Ret)), ident("r")),
			ident("nil"),
		}},
	)

	return &ast.FuncDecl{
		Name: ident(e.entryName(d.Name)),
		Type: &ast.FuncType{
			Params:  fieldList(pfields...),
			Results: fieldList(field(e.obj("Object")), field(ident("error"))),
		},
		Body: block(body...),
	}
}

// materializeShape builds the statements that turn boxed parameter p{i} into the
// Go struct the static form takes for a fixed-shape parameter. It guards the
// receiver's exact class, reads each slot out of the boxed instance through the
// public attribute read, unboxes it to its native scalar, and assembles the
// struct into x{i}. Any failure (the wrong class, an unset or unreadable slot, a
// value that does not fit its native scalar) takes the boxed fallback, so the
// struct the static body sees only ever describes an instance of the exact class
// with every slot present and representable. The fallback is a thunk so each
// early return gets its own fresh AST node.
func (e *emitter) materializeShape(i int, sh *StaticShape, pname string, fallback func() *ast.BlockStmt) []ast.Stmt {
	// The exact-class guard runs first, so the slot reads below only ever run on
	// the class whose fixed layout the struct models.
	stmts := []ast.Stmt{&ast.IfStmt{
		Cond: &ast.BinaryExpr{
			X:  callExpr(sel(pname, "TypeName")),
			Op: token.NEQ,
			Y:  strLit(sh.Name),
		},
		Body: fallback(),
	}}

	elts := make([]ast.Expr, len(sh.Fields))
	for j, f := range sh.Fields {
		rawname := fmt.Sprintf("s%d_%d", i, j)  // the boxed slot value
		valname := fmt.Sprintf("f%d_%d", i, j)  // its native scalar
		okname := fmt.Sprintf("sok%d_%d", i, j) // whether the unbox held
		stmts = append(stmts,
			// Read the slot out of the boxed instance. An error here means the slot
			// is unset or not a plain slot, which the boxed form handles.
			assign(token.DEFINE, []ast.Expr{ident(rawname), ident("err")},
				callExpr(e.obj("LoadAttr"), ident(pname), strLit(f.Name))),
			&ast.IfStmt{Cond: errNotNil(), Body: fallback()},
			// Unbox the slot to its native scalar. A spilled or mistyped value fails
			// the unbox, so a big int in an int slot leaves the static path.
			assign(token.DEFINE, []ast.Expr{ident(valname), ident(okname)},
				callExpr(e.obj(unboxAccessor(f.Scalar)), ident(rawname))),
			&ast.IfStmt{Cond: notExpr(ident(okname)), Body: fallback()},
		)
		elts[j] = kv(f.Name, ident(valname))
	}

	// Assemble the struct the static form receives.
	return append(stmts, assign(token.DEFINE,
		[]ast.Expr{ident(fmt.Sprintf("x%d", i))},
		&ast.CompositeLit{Type: ident(sh.Name), Elts: elts}))
}

// deoptHandlerName is the hand-off function the static form of a deopt-target def
// tail-calls, derived from the static form's own name so the static side and this
// side agree on it without threading a separate identifier.
func deoptHandlerName(static string) string { return static + "_deopt" }

// deoptHandlerDecl builds one def's deopt hand-off. It takes the static form's
// native parameters, reboxes each into an objects.Object, re-runs the whole unit
// through its boxed twin from the top, and returns the boxed result as the deopt
// sentinel on the error channel. A raised exception inside the twin travels the
// same channel and stays an exception, so the sentinel wrap happens only on the
// clean return. The native result slot carries a zero value the caller never
// reads, since a non-nil error always accompanies it.
func (e *emitter) deoptHandlerDecl(d *frontend.FuncDef, se StaticEntry) *ast.FuncDecl {
	n := len(se.Params)
	pfields := make([]*ast.Field, n)
	reboxed := make([]ast.Expr, n)
	for i := range n {
		pname := fmt.Sprintf("p%d", i)
		pfields[i] = field(scalarGoType(se.Params[i].Scalar), pname)
		reboxed[i] = callExpr(e.obj(reboxConstructor(se.Params[i].Scalar)), ident(pname))
	}
	sentinel := &ast.UnaryExpr{Op: token.AND, X: &ast.CompositeLit{
		Type: e.obj("Deopt"),
		Elts: []ast.Expr{kv("Value", ident("r"))},
	}}
	body := []ast.Stmt{
		assign(token.DEFINE, []ast.Expr{ident("r"), ident("err")},
			callExpr(ident(e.defName(d.Name)), reboxed...)),
		&ast.IfStmt{
			Cond: errNotNil(),
			Body: block(&ast.ReturnStmt{Results: []ast.Expr{scalarZero(se.Ret), ident("err")}}),
		},
		&ast.ReturnStmt{Results: []ast.Expr{scalarZero(se.Ret), sentinel}},
	}
	return &ast.FuncDecl{
		Name: ident(deoptHandlerName(se.Static)),
		Type: &ast.FuncType{
			Params:  fieldList(pfields...),
			Results: fieldList(field(scalarGoType(se.Ret)), field(ident("error"))),
		},
		Body: block(body...),
	}
}

// resumeHandlerDecl builds one static form's mid-loop resume hand-off. It takes
// the leading re-entry scalars (the loop counter and the live accumulator) plus
// the static form's native parameters, reboxes each into an objects.Object, and
// calls the boxed twin, which restarts the loop from that counter and
// accumulator. The twin's return is the whole unit's boxed result, returned as
// the deopt sentinel on the error channel exactly like the from-top hand-off, so
// the entry shim unwraps it the same way. Re-entering mid-loop only skips the
// iterations already run; the result the twin computes is the one the from-top
// replay would have, which is why the build gates this to the loop shape where
// that equality holds.
func (e *emitter) resumeHandlerDecl(se StaticEntry) *ast.FuncDecl {
	r := se.Resume
	// The resume hand-off reboxes the leading re-entry scalars and then the entry
	// parameters, all of which are scalars: the build gates mid-loop resume to the
	// counting-loop shape, whose entries never carry a shape parameter.
	scalars := append([]StaticScalar{}, r.Lead...)
	for _, p := range se.Params {
		scalars = append(scalars, p.Scalar)
	}
	pfields := make([]*ast.Field, len(scalars))
	reboxed := make([]ast.Expr, len(scalars))
	for i, s := range scalars {
		pname := fmt.Sprintf("p%d", i)
		pfields[i] = field(scalarGoType(s), pname)
		reboxed[i] = callExpr(e.obj(reboxConstructor(s)), ident(pname))
	}
	sentinel := &ast.UnaryExpr{Op: token.AND, X: &ast.CompositeLit{
		Type: e.obj("Deopt"),
		Elts: []ast.Expr{kv("Value", ident("r"))},
	}}
	body := []ast.Stmt{
		assign(token.DEFINE, []ast.Expr{ident("r"), ident("err")},
			callExpr(ident(r.TwinName), reboxed...)),
		&ast.IfStmt{
			Cond: errNotNil(),
			Body: block(&ast.ReturnStmt{Results: []ast.Expr{scalarZero(se.Ret), ident("err")}}),
		},
		&ast.ReturnStmt{Results: []ast.Expr{scalarZero(se.Ret), sentinel}},
	}
	return &ast.FuncDecl{
		Name: ident(r.Handler),
		Type: &ast.FuncType{
			Params:  fieldList(pfields...),
			Results: fieldList(field(scalarGoType(se.Ret)), field(ident("error"))),
		},
		Body: block(body...),
	}
}

// scalarGoType is the native Go type the static tier gives one scalar kind, the
// type the static form's parameters and result carry and the hand-off mirrors.
func scalarGoType(s StaticScalar) ast.Expr {
	switch s {
	case StaticInt:
		return ident("int64")
	case StaticFloat:
		return ident("float64")
	case StaticBool:
		return ident("bool")
	case StaticStr:
		return ident("string")
	}
	return ident("")
}

// scalarZero is the native zero of one scalar kind, the throwaway value the deopt
// hand-off returns in the native result slot next to a non-nil error.
func scalarZero(s StaticScalar) ast.Expr {
	switch s {
	case StaticInt:
		return intLit("0")
	case StaticFloat:
		return floatLit(0)
	case StaticBool:
		return ident("false")
	case StaticStr:
		return strLit("")
	}
	return ident("nil")
}

// orJoin threads one more term into a growing disjunction, returning the term
// itself for the first one so the chain starts without a nil operand.
func orJoin(acc, term ast.Expr) ast.Expr {
	if acc == nil {
		return term
	}
	return &ast.BinaryExpr{X: acc, Op: token.LOR, Y: term}
}

// unboxAccessor is the objects reader that pulls a scalar's native value out of
// its boxed form. Its second result reports success, which the guard requires
// alongside the exact-type check, so a spilled big int (AsInt false) leaves the
// static path.
func unboxAccessor(s StaticScalar) string {
	switch s {
	case StaticInt:
		return "AsInt"
	case StaticFloat:
		return "AsFloat"
	case StaticBool:
		return "AsBool"
	case StaticStr:
		return "AsStr"
	}
	return ""
}

// reboxConstructor is the objects constructor that wraps a native result back
// into its boxed form for the boxed caller.
func reboxConstructor(s StaticScalar) string {
	switch s {
	case StaticInt:
		return "NewInt"
	case StaticFloat:
		return "NewFloat"
	case StaticBool:
		return "NewBool"
	case StaticStr:
		return "NewStr"
	}
	return ""
}

// scalarTypeName is the Python type name the guard matches exactly, the string
// each scalar's boxed form reports from TypeName.
func scalarTypeName(s StaticScalar) string {
	switch s {
	case StaticInt:
		return "int"
	case StaticFloat:
		return "float"
	case StaticBool:
		return "bool"
	case StaticStr:
		return "str"
	}
	return ""
}
