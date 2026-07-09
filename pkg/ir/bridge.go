// Package ir is the seed of the typed IR pass that doc 15 describes: the layer
// between the frontend AST and pkg/emit. The full IR carries every node kind and
// a slot per expression; this first cut carries only what pkg/emit already
// lowers, the unboxed scalar subset, and translates one proven function body from
// the real AST into an emit.Func instead of a hand-built one.
//
// The rule that governs every case is the D4 safety story: this bridge only ever
// emits a static unit it can prove is a pure scalar computation. Anything outside
// the subset (an unannotated parameter, a big-int literal, a call, an attribute,
// a statement kind emit cannot lower) returns an unsupported error, and the
// caller keeps that unit boxed. The bridge never guesses a representation, so it
// can never hand emit a shape that would miscompile.
package ir

import (
	"fmt"
	"maps"
	"sort"
	"strconv"

	"github.com/tamnd/unagi/pkg/emit"
	"github.com/tamnd/unagi/pkg/frontend"
)

// unsupported marks a construct outside the scalar subset. The caller treats it
// as "keep this unit boxed", not as a compile failure, so the message names the
// construct rather than reading like an error the user must fix.
func unsupported(format string, a ...any) error {
	return fmt.Errorf("ir: unsupported for the static tier: "+format, a...)
}

// scope maps a bound name to its proven representation. It is seeded with the
// parameters and grows as the body binds locals; a name read before it is bound
// is an unsupported forward reference the bridge refuses rather than inventing a
// type for.
type scope map[string]emit.Repr

// StaticCallee describes a static-tier function a direct call may target: its
// emitted Go name and its unboxed signature. The bridge lowers a call to another
// static unit into a monomorphic Go call on this name, threading the D14 error,
// so the caller must know the callee's exact name and parameter and result
// representations to build a well-typed call.
type StaticCallee struct {
	GoName string
	Params []emit.Repr
	Ret    emit.Repr
}

// CalleeResolver reports the static callee a bare name refers to at the call
// site, or false when the name is not a static function the caller may call
// directly. A nil resolver resolves nothing, so a call always refuses and the
// unit stays boxed, which is how a caller with no known-static callees behaves.
type CalleeResolver func(name string) (StaticCallee, bool)

// SignatureOf reads a lowered function's unboxed signature into a StaticCallee
// under the given emitted Go name, so a caller's resolver can describe this
// function without re-deriving its parameter and result representations from the
// annotations. The return representation is the one the bridge inferred from the
// body, which is why it is read from the lowered function rather than the return
// annotation, since a function may omit the annotation and still lower.
func SignatureOf(f emit.Func, goName string) StaticCallee {
	params := make([]emit.Repr, len(f.Params))
	for i, p := range f.Params {
		params[i] = p.Repr
	}
	return StaticCallee{GoName: goName, Params: params, Ret: f.Ret}
}

// lowerCtx carries the ambient facts a statement needs beyond its own scope: the set
// of names read anywhere in the function, so a branch or loop can tell a live binding
// from a dead one, whether the statement sits inside a loop, so a `break` or
// `continue` is accepted only where Go would accept it, and the resolver that maps a
// callee name to its static signature. It is passed by value, so a loop can hand its
// body a copy with inLoop set without disturbing the enclosing context.
type lowerCtx struct {
	reads   map[string]bool
	inLoop  bool
	resolve CalleeResolver
}

// scalarRepr is the doc 04 representation of a scalar type named by a bare
// annotation. It is the same table emit.Of builds from a lattice type, spelled
// here against the annotation name because this seed runs before pkg/types feeds
// the bridge inferred slots.
func scalarRepr(name string) (emit.Repr, bool) {
	switch name {
	case "int":
		return emit.Repr{Go: "int64", Scalar: emit.SInt}, true
	case "float":
		return emit.Repr{Go: "float64", Scalar: emit.SFloat, Total: true}, true
	case "bool":
		return emit.Repr{Go: "bool", Scalar: emit.SBool, Total: true}, true
	case "str":
		return emit.Repr{Go: "string", Scalar: emit.SStr, Total: true}, true
	}
	return emit.Repr{}, false
}

// LowerFunc translates one frontend function into the static-tier emit model,
// reporting an unsupported error when any part of the signature or body falls
// outside the scalar subset pkg/emit lowers. On success the returned Func prints,
// through emit.EmitFunc, to the same unboxed Go the hand-built models produced.
func LowerFunc(fn *frontend.FuncDef) (emit.Func, error) {
	return LowerFuncWith(fn, nil)
}

// LowerFuncWith lowers a function with a resolver for its static-tier callees, so
// a call to another static unit lowers to a direct Go call rather than a refusal.
// A nil resolver refuses every call, which is exactly LowerFunc's behavior. The
// resolver is a pure function of the callee name, so lowering stays deterministic.
func LowerFuncWith(fn *frontend.FuncDef, resolve CalleeResolver) (emit.Func, error) {
	if fn.Async {
		return emit.Func{}, unsupported("async def %s", fn.Name)
	}
	if len(fn.Decorators) != 0 {
		return emit.Func{}, unsupported("decorated def %s", fn.Name)
	}

	sc := scope{}
	params := make([]emit.Param, len(fn.Params))
	for i, p := range fn.Params {
		if p.Kind != frontend.ParamPlain && p.Kind != frontend.ParamPosOnly {
			return emit.Func{}, unsupported("parameter %s is not a plain positional parameter", p.Name)
		}
		if p.Default != nil {
			return emit.Func{}, unsupported("parameter %s has a default", p.Name)
		}
		if p.Annotation == nil {
			return emit.Func{}, unsupported("parameter %s is unannotated", p.Name)
		}
		r, ok := annotationRepr(p.Annotation)
		if !ok {
			return emit.Func{}, unsupported("parameter %s has a non-scalar annotation", p.Name)
		}
		params[i] = emit.Param{Name: p.Name, Repr: r}
		sc[p.Name] = r
	}

	// A join local is declared ahead of the branch that assigns it, so before lowering
	// the body the bridge learns which names are read anywhere in the function. A name
	// an arm binds but nothing ever reads has no live static form: its Go declaration
	// would be written and never used, which does not compile, so lowerIf keeps such a
	// unit boxed rather than emit a local Go rejects.
	reads := map[string]bool{}
	loadedNames(fn.Body, reads)
	ctx := lowerCtx{reads: reads, resolve: resolve}

	body, ret, terminates, err := lowerBody(fn.Body, sc, ctx)
	if err != nil {
		return emit.Func{}, err
	}
	if ret == nil {
		return emit.Func{}, unsupported("%s has no return the tier can type", fn.Name)
	}
	// Every path has to return a scalar. A body that can fall off its end returns
	// Python None there, which is not a scalar this tier represents, and the emitted
	// Go would also miss its terminating return, so a non-exhaustive body stays
	// boxed rather than lowering to a shape that neither types nor compiles.
	if !terminates {
		return emit.Func{}, unsupported("%s can fall off its end without returning", fn.Name)
	}

	// A declared return annotation, when present and scalar, must agree with what
	// the body actually returns. A mismatch means inference and the annotation
	// disagree, which is exactly the case D4 says to keep boxed rather than trust.
	if fn.Returns != nil {
		want, ok := annotationRepr(fn.Returns)
		if !ok {
			return emit.Func{}, unsupported("%s has a non-scalar return annotation", fn.Name)
		}
		if want.Scalar != ret.Scalar {
			return emit.Func{}, unsupported("%s returns %s but is annotated %s", fn.Name, ret.Scalar, want.Scalar)
		}
	}

	return emit.Func{Name: fn.Name, Params: params, Ret: *ret, Body: body}, nil
}

// annotationRepr reads a bare-name scalar annotation. Only `int`, `float`,
// `bool`, and `str` written as a plain name resolve; a qualified, subscripted, or
// unknown annotation is not a scalar this tier represents.
func annotationRepr(e frontend.Expr) (emit.Repr, bool) {
	name, ok := e.(*frontend.Name)
	if !ok {
		return emit.Repr{}, false
	}
	return scalarRepr(name.Id)
}

// lowerBody translates a statement block and reports the representation the block
// returns and whether the block is exhaustive, meaning control always returns
// before running off its end. Every return in the block must agree on the
// representation, so the emitted Go has one result type; a block that returns two
// different scalar classes is beyond this seed and stays boxed. An arm binds into
// its own forked scope, and lowerIf reconciles the two arms at the join, so a name
// both arms bind becomes one hoisted Go local rather than two shadowing `:=` writes
// (doc 06 section 8, the join rule).
func lowerBody(stmts []frontend.Stmt, sc scope, ctx lowerCtx) ([]emit.Stmt, *emit.Repr, bool, error) {
	var out []emit.Stmt
	var ret *emit.Repr
	var terminates bool
	for _, s := range stmts {
		es, rr, term, err := lowerStmt(s, sc, ctx)
		if err != nil {
			return nil, nil, false, err
		}
		if rr != nil {
			if ret != nil && ret.Scalar != rr.Scalar {
				return nil, nil, false, unsupported("return type is %s on one path and %s on another", ret.Scalar, rr.Scalar)
			}
			ret = rr
		}
		if term {
			terminates = true
		}
		out = append(out, es...)
	}
	return out, ret, terminates, nil
}

// lowerStmt translates one statement. The second result is non-nil only for a
// return or an if whose arms both return, carrying the representation the path
// yields so lowerBody can pin the function's result type. The third result reports
// whether the statement is exhaustive: a return always is, an if is exactly when it
// has an else and both arms are, and every other form is not.
func lowerStmt(s frontend.Stmt, sc scope, ctx lowerCtx) ([]emit.Stmt, *emit.Repr, bool, error) {
	switch n := s.(type) {
	case *frontend.Pass:
		return nil, nil, false, nil

	case *frontend.Break:
		// The parser accepts a stray break, so the bridge is the gate: a break outside a
		// loop has no Go form here and keeps the unit boxed rather than emit an invalid
		// jump. Inside a loop it lowers to Go's break and does not itself terminate the
		// function, since control leaves the loop, not the function.
		if !ctx.inLoop {
			return nil, nil, false, unsupported("break outside a loop has no static form")
		}
		return []emit.Stmt{emit.Break{}}, nil, false, nil

	case *frontend.Continue:
		if !ctx.inLoop {
			return nil, nil, false, unsupported("continue outside a loop has no static form")
		}
		return []emit.Stmt{emit.Continue{}}, nil, false, nil

	case *frontend.Return:
		if n.Value == nil {
			return nil, nil, false, unsupported("a bare return has no scalar value")
		}
		v, r, err := lowerExpr(n.Value, sc, ctx)
		if err != nil {
			return nil, nil, false, err
		}
		return []emit.Stmt{emit.Return{Value: v}}, &r, true, nil

	case *frontend.If:
		return lowerIf(n, sc, ctx)

	case *frontend.While:
		return lowerWhile(n, sc, ctx)

	case *frontend.For:
		return lowerFor(n, sc, ctx)

	case *frontend.Assign:
		if len(n.Targets) != 1 {
			return nil, nil, false, unsupported("chained assignment")
		}
		if tup, ok := n.Targets[0].(*frontend.TupleLit); ok {
			return lowerTupleAssign(tup, n.Value, sc, ctx)
		}
		name, ok := n.Targets[0].(*frontend.Name)
		if !ok {
			return nil, nil, false, unsupported("assignment target is not a plain name")
		}
		v, r, err := lowerExpr(n.Value, sc, ctx)
		if err != nil {
			return nil, nil, false, err
		}
		if prev, bound := sc[name.Id]; bound {
			// Rebinding an existing name is a plain assignment, not a second declaration.
			// Go fixes a variable's type at its declaration, so a rebinding to the same
			// scalar reassigns it, and CPython's dynamic rebinding to a different type has
			// no static Go form, so a type-changing rebind keeps the unit boxed.
			if prev.Scalar != r.Scalar {
				return nil, nil, false, unsupported("%s rebinds a %s value as a %s, which Go cannot express", name.Id, prev.Scalar, r.Scalar)
			}
			sc[name.Id] = r
			return []emit.Stmt{emit.Assign{Name: name.Id, Value: v}}, nil, false, nil
		}
		sc[name.Id] = r
		return []emit.Stmt{emit.Define{Name: name.Id, Value: v}}, nil, false, nil

	case *frontend.AnnAssign:
		name, ok := n.Target.(*frontend.Name)
		if !ok {
			return nil, nil, false, unsupported("annotated assignment target is not a plain name")
		}
		if n.Value == nil {
			return nil, nil, false, unsupported("bare annotation without a value binds nothing")
		}
		v, r, err := lowerExpr(n.Value, sc, ctx)
		if err != nil {
			return nil, nil, false, err
		}
		if want, ok := annotationRepr(n.Annotation); ok && want.Scalar != r.Scalar {
			return nil, nil, false, unsupported("%s is annotated %s but bound a %s", name.Id, want.Scalar, r.Scalar)
		}
		// A first annotated binding declares the name; re-annotating an already-bound
		// name would emit a second `:=`, invalid Go, so it reassigns when the scalar
		// agrees and stays boxed otherwise, the same rule the plain assignment follows.
		if prev, bound := sc[name.Id]; bound {
			if prev.Scalar != r.Scalar {
				return nil, nil, false, unsupported("%s rebinds a %s value as a %s, which Go cannot express", name.Id, prev.Scalar, r.Scalar)
			}
			sc[name.Id] = r
			return []emit.Stmt{emit.Assign{Name: name.Id, Value: v}}, nil, false, nil
		}
		sc[name.Id] = r
		return []emit.Stmt{emit.Define{Name: name.Id, Value: v}}, nil, false, nil

	case *frontend.AugAssign:
		// Only +=, -=, *= lower here: they keep the target's representation, so the
		// accumulator's type is unchanged. /= would promote an int target to float
		// (a rebinding), and the rest are outside the scalar seed (binOp refuses them).
		if n.Op != frontend.BinAdd && n.Op != frontend.BinSub && n.Op != frontend.BinMul {
			return nil, nil, false, unsupported("augmented assignment other than +=, -=, *=")
		}
		op, _ := binOp(n.Op)
		name, ok := n.Target.(*frontend.Name)
		if !ok {
			return nil, nil, false, unsupported("augmented assignment target is not a plain name")
		}
		tr, ok := sc[name.Id]
		if !ok {
			return nil, nil, false, unsupported("%s %s= reads %s before it is bound", name.Id, op, name.Id)
		}
		v, vr, err := lowerExpr(n.Value, sc, ctx)
		if err != nil {
			return nil, nil, false, err
		}
		if _, err := binResult(op, tr, vr); err != nil {
			return nil, nil, false, err
		}
		return []emit.Stmt{emit.AugAssign{Name: name.Id, Op: op, Repr: tr, Value: v}}, nil, false, nil

	case *frontend.ExprStmt:
		// A bare expression statement: a docstring, a constant on a line, or a call for
		// its effect. Lowering the value first validates it the same as any expression,
		// so an unbound name or an unsupported operator refuses here and boxes the unit
		// rather than dropping a statement that could raise. A value that is pure and
		// cannot raise (a literal, a bare name read) has no observable effect once its
		// result is discarded, so it lowers to no statement at all: this is what lets a
		// function whose first line is a docstring stay static. Anything else (a call
		// that can raise, a division with its zero check) becomes a Discard so its
		// effect runs and its exception still propagates, even though the value is unused.
		v, _, err := lowerExpr(n.X, sc, ctx)
		if err != nil {
			return nil, nil, false, err
		}
		if pureDiscardable(v) {
			return nil, nil, false, nil
		}
		return []emit.Stmt{emit.Discard{Value: v}}, nil, false, nil
	}
	return nil, nil, false, unsupported("statement %T", s)
}

// pureDiscardable reports whether an emit expression has no observable effect and
// cannot raise, so discarding its result on a bare statement line lowers to nothing.
// A literal and a bare variable read qualify; a call, an arithmetic operation with an
// overflow guard, and a division with its zero check do not, since each either runs
// an effect or can raise and so must keep its Discard statement.
func pureDiscardable(e emit.Expr) bool {
	switch e.(type) {
	case emit.Int, emit.Float, emit.Bool, emit.Str, emit.Var:
		return true
	}
	return false
}

// lowerTupleAssign lowers a scalar tuple unpack `x, y = a, b`. The right side must
// be a tuple literal of the same length, since unpacking an iterable value has no
// static form at M4 and stays boxed, and every target must be a distinct plain
// name. Go's parallel assignment evaluates the whole right side before binding any
// target, the same order Python's unpack uses, so a swap needs no temp and each
// value is read exactly once. Either every target is a fresh binding (a parallel
// Define) or every target rebinds an existing name of the same scalar (a parallel
// Assign); a mix, or a type-changing rebind, keeps the unit boxed because Go's :=
// and = do not compose across a half-new left side.
func lowerTupleAssign(tgt *frontend.TupleLit, value frontend.Expr, sc scope, ctx lowerCtx) ([]emit.Stmt, *emit.Repr, bool, error) {
	rhs, ok := value.(*frontend.TupleLit)
	if !ok {
		return nil, nil, false, unsupported("tuple unpack of a non-tuple value stays boxed")
	}
	if len(tgt.Elts) != len(rhs.Elts) {
		return nil, nil, false, unsupported("tuple unpack binds %d names to %d values", len(tgt.Elts), len(rhs.Elts))
	}
	if len(tgt.Elts) < 2 {
		return nil, nil, false, unsupported("a tuple unpack needs at least two targets")
	}
	names := make([]string, len(tgt.Elts))
	seen := map[string]bool{}
	for i, t := range tgt.Elts {
		nm, ok := t.(*frontend.Name)
		if !ok {
			return nil, nil, false, unsupported("tuple unpack target is not a plain name")
		}
		if seen[nm.Id] {
			return nil, nil, false, unsupported("tuple unpack repeats the target %s, which Go's parallel assignment forbids", nm.Id)
		}
		seen[nm.Id] = true
		names[i] = nm.Id
	}
	// Lower every value before binding any name, so a value that reads a target sees
	// its pre-assignment binding, exactly Python's evaluate-then-bind order.
	vals := make([]emit.Expr, len(rhs.Elts))
	reprs := make([]emit.Repr, len(rhs.Elts))
	for i, e := range rhs.Elts {
		v, r, err := lowerExpr(e, sc, ctx)
		if err != nil {
			return nil, nil, false, err
		}
		vals[i], reprs[i] = v, r
	}
	fresh, rebind := 0, 0
	for i, name := range names {
		if prev, bound := sc[name]; bound {
			if prev.Scalar != reprs[i].Scalar {
				return nil, nil, false, unsupported("%s rebinds a %s value as a %s, which Go cannot express", name, prev.Scalar, reprs[i].Scalar)
			}
			rebind++
		} else {
			fresh++
		}
	}
	if fresh > 0 && rebind > 0 {
		return nil, nil, false, unsupported("a tuple unpack that mixes fresh and rebound names stays boxed")
	}
	for i, name := range names {
		sc[name] = reprs[i]
	}
	return []emit.Stmt{emit.Bind{Names: names, Values: vals, Define: fresh > 0}}, nil, false, nil
}

// lowerIf translates an if/elif/else chain, materializing the branch join doc 06
// section 8 describes. The condition is any scalar the truthiness rule accepts. Each
// arm lowers into its own forked scope, so a name an arm binds does not leak into the
// sibling arm or the condition. A name both arms bind to the same scalar joins to one
// Go local hoisted ahead of the branch, and each arm assigns it rather than
// redeclaring, so the value the taken arm writes is the one read after the block. A
// name only one arm binds stays inside that arm and is not visible after; a later read
// of it is refused as an unbound name, so no untyped Go zero leaks past the branch. A
// scalar that disagrees across the arms has no single Go type and keeps the unit boxed.
// The chain is exhaustive only when it has an else and both arms are, which is what
// lets the function demand a return on every path. An elif rides in as a nested If in
// Else, so the recursion here produces the nested emit.If the emitter folds to else-if.
func lowerIf(n *frontend.If, sc scope, ctx lowerCtx) ([]emit.Stmt, *emit.Repr, bool, error) {
	cond, cr, err := lowerExpr(n.Cond, sc, ctx)
	if err != nil {
		return nil, nil, false, err
	}
	if !truthy(cr) {
		return nil, nil, false, unsupported("no truthiness form for a %s condition", cr.Scalar)
	}
	hasElse := len(n.Else) > 0

	// Discovery pass: lower both arms into forked scopes to learn which names each
	// binds. The lowered statements are discarded; only the bound names, the returned
	// representations, and whether each arm terminates are kept. The arms re-lower once
	// the join names are known, so a first binding of a join name emits an assignment
	// to the hoisted local rather than a fresh declaration.
	thenSc := cloneScope(sc)
	_, thenRet, thenTerm, err := lowerBody(n.Body, thenSc, ctx)
	if err != nil {
		return nil, nil, false, err
	}
	elseSc := cloneScope(sc)
	var elseRet *emit.Repr
	elseTerm := false
	if hasElse {
		_, elseRet, elseTerm, err = lowerBody(n.Else, elseSc, ctx)
		if err != nil {
			return nil, nil, false, err
		}
	}
	thenNew := newBindings(sc, thenSc)
	elseNew := newBindings(sc, elseSc)

	// A name both arms bind is a join candidate, but only when the two arms give it the
	// same scalar; a scalar mismatch across the arms has no single Go type, so the join
	// is refused and the unit stays boxed.
	var joinNames []string
	joinRepr := map[string]emit.Repr{}
	for name, tr := range thenNew {
		er, both := elseNew[name]
		if !both {
			continue
		}
		if tr.Scalar != er.Scalar {
			return nil, nil, false, unsupported("%s joins as %s on one arm and %s on the other, which Go cannot type", name, tr.Scalar, er.Scalar)
		}
		joinNames = append(joinNames, name)
		joinRepr[name] = tr
	}
	sort.Strings(joinNames)

	// Every name an arm binds must be read somewhere, or its Go declaration would be
	// written and never used, which does not compile. A join name never read is dead in
	// both arms; an arm-only name never read is dead in its arm. Either way the unit
	// stays boxed rather than emit a local Go rejects.
	for name := range thenNew {
		if !ctx.reads[name] {
			return nil, nil, false, unsupported("%s is bound in a branch but never read, so it has no live static form", name)
		}
	}
	for name := range elseNew {
		if !ctx.reads[name] {
			return nil, nil, false, unsupported("%s is bound in a branch but never read, so it has no live static form", name)
		}
	}

	// Re-lower each arm with the join names pre-seeded, so the arm reassigns the hoisted
	// local (an emit.Assign) instead of declaring a fresh one, and a nested branch that
	// also binds a join name reassigns rather than redeclares it too.
	thenSc2 := cloneScope(sc)
	for _, name := range joinNames {
		thenSc2[name] = joinRepr[name]
	}
	then, _, _, err := lowerBody(n.Body, thenSc2, ctx)
	if err != nil {
		return nil, nil, false, err
	}
	var els []emit.Stmt
	if hasElse {
		elseSc2 := cloneScope(sc)
		for _, name := range joinNames {
			elseSc2[name] = joinRepr[name]
		}
		els, _, _, err = lowerBody(n.Else, elseSc2, ctx)
		if err != nil {
			return nil, nil, false, err
		}
	}

	// The join names outlive the branch, so each is declared ahead of it and added to
	// the enclosing scope. The declaration gives the zero value, but every arm assigns
	// the name on its path, so the zero is never observed once the branch is exhaustive.
	var out []emit.Stmt
	for _, name := range joinNames {
		out = append(out, emit.VarDecl{Name: name, Repr: joinRepr[name]})
		sc[name] = joinRepr[name]
	}
	out = append(out, emit.If{Cond: cond, Then: then, Else: els})

	ret, err := joinReturns(thenRet, elseRet)
	if err != nil {
		return nil, nil, false, err
	}
	return out, ret, thenTerm && hasElse && elseTerm, nil
}

// lowerWhile translates a `while cond:` loop to a Go `for cond {}`. The condition is
// any scalar the truthiness rule accepts, lowered the same way an if condition is. The
// body forks a loop scope so a name the body binds fresh stays loop-local and does not
// leak past the loop; a body that rebinds an outer name reassigns the outer variable,
// which the accumulator pattern relies on.
//
// A guard in the body deopts to the boxed twin, which re-runs the whole unit boxed
// from the top. The static subset is effect-free, so that from-top replay reaches the
// same result the mid-iteration state would have, which makes a guarded body sound at
// M4; the mid-loop back-edge resume that skips the redone iterations is a later
// performance slice. A guard in the condition still keeps the unit boxed: it fires
// before the body runs and its cheapest resume is the same back-edge, so it waits for
// that slice too. A `while ... else` runs its else when the loop exits without a break;
// doc 06 line 40 keeps that boxed at M4, so a non-empty else is refused here too.
//
// A while may run zero times or loop without returning, so it never makes the function
// exhaustive; it reports term false and carries no result representation.
func lowerWhile(n *frontend.While, sc scope, ctx lowerCtx) ([]emit.Stmt, *emit.Repr, bool, error) {
	if len(n.Else) > 0 {
		return nil, nil, false, unsupported("a while-else has no static form at M4")
	}
	cond, cr, err := lowerExpr(n.Cond, sc, ctx)
	if err != nil {
		return nil, nil, false, err
	}
	if !truthy(cr) {
		return nil, nil, false, unsupported("no truthiness form for a %s condition", cr.Scalar)
	}
	// A guard in the condition fires every iteration and has no back-edge resume point
	// yet, so a guarded condition keeps the unit boxed.
	var cc Cost
	costExpr(cond, &cc)
	if cc.EntryGuards+cc.LoopGuards > 0 {
		return nil, nil, false, unsupported("a guarded while condition needs a loop back-edge resume point, deferred past M4")
	}
	// The body lowers into a forked loop scope, marked inLoop so a break or continue
	// finds its loop. New bindings stay in the fork and do not reach the enclosing scope.
	loopSc := cloneScope(sc)
	bodyCtx := ctx
	bodyCtx.inLoop = true
	body, _, _, err := lowerBody(n.Body, loopSc, bodyCtx)
	if err != nil {
		return nil, nil, false, err
	}
	// An overflow guard in the body deopts to the boxed twin, which re-runs the whole
	// unit boxed from the top. The static subset is effect-free, so that from-top
	// replay recomputes the same result the mid-loop state would have reached, only
	// slower, which is sound; the mid-loop back-edge resume that skips the redone
	// iterations is a later performance slice. An observable effect before a guard
	// would break the replay, but the deopt plan's VerifyPlan gate demotes such a
	// unit to boxed before it can ship, so the body's guards are safe to admit here.
	return []emit.Stmt{emit.While{Cond: cond, Body: body}}, nil, false, nil
}

// lowerFor translates a `for i in range(...)` counting loop to a Go
// `for i := start; i < stop; i++`. The induction variable is int64, so the body reads
// it as an int, and the loop is the canonical unboxed counting loop doc 06 calls the
// single most important lowering in the compiler.
//
// The range forms that lower here count by one: `range(n)` counts from zero, `range(a, b)`
// counts from a, and `range(a, b, step)` counts from a with the step's direction, all to
// stop exclusive. The step must be a literal of magnitude one so its sign is known and the
// induction cannot overflow before the bound test fires: a `+1` counts up, a `-1` counts
// down, and any larger step keeps the unit boxed (doc 06 line 46). An enumerate or a list
// iteration with a non-name target (doc 06 line 48) is a later slice too, so a target that
// is not a plain name or an iterable that is not a range call keeps the unit boxed.
//
// The stop bound is re-tested every iteration, so it must be stable: an int literal or a
// name the body never reassigns, or a later iteration would test a changed bound where
// Python captured the bound once at loop entry. A computed bound is hoisted into a fresh
// temp evaluated once ahead of the loop (doc 06 line 50), so the header tests a plain
// int64 local and any guard the bound carries stays at function entry. Python also rebinds
// the loop variable from the range each iteration and ignores a body assignment to it, so a body
// that assigns the loop variable, or a loop variable that shadows an outer binding, has
// no faithful Go counting-loop form and stays boxed. As with while, a guard in the body
// deopts to the boxed twin and re-runs the unit boxed from the top; the effect-free
// static subset makes that from-top replay reach the same result, so a guarded body is
// admitted here and the mid-loop back-edge resume is a later performance slice.
//
// A for may run zero times, so it never makes the function exhaustive; it reports term
// false and carries no result representation. New body bindings and the loop variable
// stay in a forked scope and do not reach the enclosing scope.
func lowerFor(n *frontend.For, sc scope, ctx lowerCtx) ([]emit.Stmt, *emit.Repr, bool, error) {
	if len(n.Else) > 0 {
		return nil, nil, false, unsupported("a for-else has no static form at M4")
	}
	target, ok := n.Target.(*frontend.Name)
	if !ok {
		return nil, nil, false, unsupported("a for target that is not a plain name has no counting-loop form yet")
	}
	if _, exists := sc[target.Id]; exists {
		return nil, nil, false, unsupported("the loop variable %s shadows an outer binding, which Go and Python leave in different states after the loop", target.Id)
	}
	call, ok := n.Iter.(*frontend.Call)
	if !ok {
		return nil, nil, false, unsupported("a for over a non-range iterable has no counting-loop form yet")
	}
	fnName, ok := call.Fn.(*frontend.Name)
	if !ok || fnName.Id != "range" {
		return nil, nil, false, unsupported("a for over a non-range call has no counting-loop form yet")
	}
	for _, a := range call.Args {
		if a.Star != 0 || a.Name != "" {
			return nil, nil, false, unsupported("range with a keyword or star argument has no static form")
		}
	}
	var startArg, stopArg frontend.Expr
	var down bool
	switch len(call.Args) {
	case 1:
		stopArg = call.Args[0].Value
	case 2:
		startArg = call.Args[0].Value
		stopArg = call.Args[1].Value
	case 3:
		startArg = call.Args[0].Value
		stopArg = call.Args[1].Value
		// The step must be a literal so its sign, and so the loop's termination direction,
		// is known at compile time. Only a step of magnitude one lands here: a `+1` counts
		// up like the two-argument form, a `-1` counts down, and any larger step could
		// carry the induction past int64's range before the bound test fires, an overflow
		// guard on the loop back-edge with no resume point yet, so it stays boxed. A zero
		// step is a Python ValueError and stays boxed too.
		step, ok := constStep(call.Args[2].Value)
		if !ok {
			return nil, nil, false, unsupported("a range step that is not an integer literal has no compile-time sign, so the loop direction is unknown")
		}
		switch step {
		case 1:
			down = false
		case -1:
			down = true
		default:
			return nil, nil, false, unsupported("a range step of magnitude other than one needs a loop back-edge overflow guard, deferred past M4")
		}
	default:
		return nil, nil, false, unsupported("range needs one to three arguments")
	}

	// The start is evaluated once in the loop init, so any int expression serves.
	var start emit.Expr
	if startArg == nil {
		start = emit.Int{V: 0}
	} else {
		s, sr, err := lowerExpr(startArg, sc, ctx)
		if err != nil {
			return nil, nil, false, err
		}
		if sr.Scalar != emit.SInt {
			return nil, nil, false, unsupported("a range start must be an int, got %s", sr.Scalar)
		}
		start = s
	}

	// The stop bound is re-tested each iteration, so it must be a stable int: a literal,
	// or a name the body never reassigns. Anything computed needs the hoisted temp of a
	// later slice.
	stop, stopRepr, err := lowerExpr(stopArg, sc, ctx)
	if err != nil {
		return nil, nil, false, err
	}
	if stopRepr.Scalar != emit.SInt {
		return nil, nil, false, unsupported("a range stop must be an int, got %s", stopRepr.Scalar)
	}
	var pre []emit.Stmt
	switch b := stopArg.(type) {
	case *frontend.IntLit:
	case *frontend.Name:
		if bodyAssigns(n.Body, b.Id) {
			return nil, nil, false, unsupported("the loop body reassigns the range bound %s, which the counting loop would re-read", b.Id)
		}
	default:
		// A computed bound is evaluated once at loop entry, exactly as Python evaluates a
		// range argument once. Go would re-run the stop expression on every back-edge, which
		// both repeats the work and, for a guarded int expression, moves the overflow guard
		// onto the loop back-edge where no resume point exists yet. Hoisting the bound into a
		// fresh temp ahead of the loop evaluates it once and keeps any guard it carries at
		// function entry, so the loop header only tests a plain int64 local.
		tmp := freshLocal(sc, n.Body, target.Id, "bound")
		pre = append(pre, emit.Define{Name: tmp, Value: stop})
		stop = emit.Var{Name: tmp, Repr: stopRepr}
	}

	if bodyAssigns(n.Body, target.Id) {
		return nil, nil, false, unsupported("the loop body reassigns the loop variable %s, which perturbs a Go counting loop", target.Id)
	}

	loopSc := cloneScope(sc)
	loopSc[target.Id] = emit.Repr{Go: "int64", Scalar: emit.SInt}
	bodyCtx := ctx
	bodyCtx.inLoop = true
	body, _, _, err := lowerBody(n.Body, loopSc, bodyCtx)
	if err != nil {
		return nil, nil, false, err
	}
	// A body overflow guard deopts to the boxed twin, which re-runs the unit boxed
	// from the top. That from-top replay is sound because the static subset is
	// effect-free, so recomputing the redone iterations reaches the same result; the
	// mid-loop back-edge resume that avoids the rework is a later performance slice,
	// and the VerifyPlan gate demotes any unit whose plan gains an effect before a
	// guard, so admitting the body's guards here cannot ship a wrong answer.
	return append(pre, emit.ForCount{Var: target.Id, Start: start, Stop: stop, Down: down, Body: body}), nil, false, nil
}

// constStep reads a range step written as an integer literal, optionally negated once, and
// returns its signed value. Only a literal step has a compile-time-known sign, which the
// counting loop needs to pick its termination direction, so a name or a computed step
// reports ok false and keeps the loop boxed.
func constStep(e frontend.Expr) (int64, bool) {
	neg := false
	if u, ok := e.(*frontend.UnaryOp); ok {
		if u.Op != frontend.UnaryNeg {
			return 0, false
		}
		neg = true
		e = u.X
	}
	lit, ok := e.(*frontend.IntLit)
	if !ok {
		return 0, false
	}
	v, err := strconv.ParseInt(lit.Text, 10, 64)
	if err != nil {
		return 0, false
	}
	if neg {
		v = -v
	}
	return v, true
}

// bodyAssigns reports whether any statement in a block assigns the given name, walking
// nested branches and loops. It is the soundness gate the counting loop leans on: a
// range bound or a loop variable a body reassigns cannot lower to a Go counting loop
// whose header re-reads the bound and drives the induction. A store position it does not
// recognize is not counted, which can only make the loop refuse more, never miscompile.
func bodyAssigns(stmts []frontend.Stmt, name string) bool {
	for _, s := range stmts {
		switch n := s.(type) {
		case *frontend.Assign:
			for _, t := range n.Targets {
				if targetHits(t, name) {
					return true
				}
			}
		case *frontend.AnnAssign:
			if targetHits(n.Target, name) {
				return true
			}
		case *frontend.AugAssign:
			if targetHits(n.Target, name) {
				return true
			}
		case *frontend.If:
			if bodyAssigns(n.Body, name) || bodyAssigns(n.Else, name) {
				return true
			}
		case *frontend.While:
			if bodyAssigns(n.Body, name) || bodyAssigns(n.Else, name) {
				return true
			}
		case *frontend.For:
			if targetHits(n.Target, name) || bodyAssigns(n.Body, name) || bodyAssigns(n.Else, name) {
				return true
			}
		}
	}
	return false
}

// freshLocal returns a Go local name built from prefix that no live binding uses, the
// loop variable does not take, and the loop body never assigns, so a hoisted temp cannot
// shadow a name the body reads or clash with an outer binding. It tries the bare prefix
// first, then the prefix with a rising counter, so the name is a deterministic function
// of what is already taken and two runs over the same loop pick the same temp.
func freshLocal(sc scope, body []frontend.Stmt, loopVar, prefix string) string {
	for i := 0; ; i++ {
		cand := prefix
		if i > 0 {
			cand = fmt.Sprintf("%s%d", prefix, i)
		}
		if _, exists := sc[cand]; exists {
			continue
		}
		if cand == loopVar || bodyAssigns(body, cand) {
			continue
		}
		return cand
	}
}

// targetHits reports whether an assignment target binds the given name, descending into
// a tuple target so `a, b = ...` and a tuple loop target are seen.
func targetHits(e frontend.Expr, name string) bool {
	switch t := e.(type) {
	case *frontend.Name:
		return t.Id == name
	case *frontend.TupleLit:
		for _, el := range t.Elts {
			if targetHits(el, name) {
				return true
			}
		}
	}
	return false
}

// cloneScope copies a scope so an arm can bind into it without disturbing the
// enclosing scope; the two arms discover their bindings independently this way.
func cloneScope(sc scope) scope {
	out := make(scope, len(sc))
	maps.Copy(out, sc)
	return out
}

// newBindings reports the names a forked arm scope bound that the parent scope did
// not carry, with the representation each was bound to, so lowerIf can tell a fresh
// binding from a rebinding of an outer name.
func newBindings(parent, child scope) map[string]emit.Repr {
	out := map[string]emit.Repr{}
	for name, r := range child {
		if _, existed := parent[name]; !existed {
			out[name] = r
		}
	}
	return out
}

// loadedNames collects every name read somewhere in a block, walking into nested
// branches. A binding whose name never appears here is written and never read, so a
// join local declared for it would not compile; lowerIf uses this set to keep such a
// unit boxed. Only genuine load positions are counted: an assignment target is a
// store, not a load, so a name that is only ever assigned is correctly absent. A node
// kind this walk does not recognize contributes no loads, which can only make the set
// smaller and so only ever keeps more units boxed, never fewer.
func loadedNames(stmts []frontend.Stmt, out map[string]bool) {
	for _, s := range stmts {
		switch n := s.(type) {
		case *frontend.Return:
			loadedInExpr(n.Value, out)
		case *frontend.ExprStmt:
			loadedInExpr(n.X, out)
		case *frontend.Assign:
			loadedInExpr(n.Value, out)
		case *frontend.AnnAssign:
			loadedInExpr(n.Value, out)
		case *frontend.AugAssign:
			// `x += v` reads x before it writes it, so the target is a load here as well.
			if nm, ok := n.Target.(*frontend.Name); ok {
				out[nm.Id] = true
			}
			loadedInExpr(n.Value, out)
		case *frontend.If:
			loadedInExpr(n.Cond, out)
			loadedNames(n.Body, out)
			loadedNames(n.Else, out)
		case *frontend.While:
			loadedInExpr(n.Cond, out)
			loadedNames(n.Body, out)
			loadedNames(n.Else, out)
		case *frontend.For:
			loadedInExpr(n.Iter, out)
			loadedNames(n.Body, out)
			loadedNames(n.Else, out)
		}
	}
}

// loadedInExpr walks the load positions of one expression, recording every name it
// reads. It understands the scalar-subset nodes the bridge lowers; any other node
// contributes nothing, which only shrinks the read set and so only keeps more units
// boxed.
func loadedInExpr(e frontend.Expr, out map[string]bool) {
	switch n := e.(type) {
	case *frontend.Name:
		out[n.Id] = true
	case *frontend.BinOp:
		loadedInExpr(n.Left, out)
		loadedInExpr(n.Right, out)
	case *frontend.UnaryOp:
		loadedInExpr(n.X, out)
	case *frontend.BoolOp:
		for _, v := range n.Values {
			loadedInExpr(v, out)
		}
	case *frontend.Compare:
		loadedInExpr(n.Left, out)
		for _, r := range n.Rights {
			loadedInExpr(r, out)
		}
	case *frontend.TupleLit:
		for _, el := range n.Elts {
			loadedInExpr(el, out)
		}
	case *frontend.ListLit:
		for _, el := range n.Elts {
			loadedInExpr(el, out)
		}
	}
}

// truthy reports whether a representation has a static truthiness form, the rule
// emit.truthyExpr lowers. Every scalar does; an aggregate does through its length,
// though the bridge carries no aggregate condition operand yet.
func truthy(r emit.Repr) bool {
	switch r.Scalar {
	case emit.SBool, emit.SInt, emit.SFloat, emit.SStr:
		return true
	}
	return r.Elem != nil
}

// joinReturns reconciles the representations two branches return. A branch that
// does not return contributes nothing; when both return they must agree on the
// scalar class, since the function has one result type and a divergent join is the
// case doc 06 keeps boxed rather than widening.
func joinReturns(then, els *emit.Repr) (*emit.Repr, error) {
	if then == nil {
		return els, nil
	}
	if els == nil {
		return then, nil
	}
	if then.Scalar != els.Scalar {
		return nil, unsupported("if returns %s on one arm and %s on the other", then.Scalar, els.Scalar)
	}
	return then, nil
}

// lowerExpr translates one expression and returns its representation alongside
// the emit node, so the caller always knows the shape of the value without a
// second inference pass.
func lowerExpr(e frontend.Expr, sc scope, ctx lowerCtx) (emit.Expr, emit.Repr, error) {
	switch n := e.(type) {
	case *frontend.Name:
		r, ok := sc[n.Id]
		if !ok {
			return nil, emit.Repr{}, unsupported("name %s is read before it is bound", n.Id)
		}
		return emit.Var{Name: n.Id, Repr: r}, r, nil

	case *frontend.IntLit:
		v, err := strconv.ParseInt(n.Text, 10, 64)
		if err != nil {
			return nil, emit.Repr{}, unsupported("integer literal %s does not fit int64", n.Text)
		}
		return emit.Int{V: v}, emit.Repr{Go: "int64", Scalar: emit.SInt}, nil

	case *frontend.FloatLit:
		return emit.Float{V: n.Val}, emit.Repr{Go: "float64", Scalar: emit.SFloat, Total: true}, nil

	case *frontend.BoolLit:
		return emit.Bool{V: n.Val}, emit.Repr{Go: "bool", Scalar: emit.SBool, Total: true}, nil

	case *frontend.StrLit:
		return emit.Str{V: n.Val}, emit.Repr{Go: "string", Scalar: emit.SStr, Total: true}, nil

	case *frontend.BinOp:
		op, ok := binOp(n.Op)
		if !ok {
			return nil, emit.Repr{}, unsupported("binary operator %v", n.Op)
		}
		l, lr, err := lowerExpr(n.Left, sc, ctx)
		if err != nil {
			return nil, emit.Repr{}, err
		}
		r, rr, err := lowerExpr(n.Right, sc, ctx)
		if err != nil {
			return nil, emit.Repr{}, err
		}
		res, err := binResult(op, lr, rr)
		if err != nil {
			return nil, emit.Repr{}, err
		}
		return emit.Bin{Op: op, L: l, R: r}, res, nil

	case *frontend.Compare:
		return lowerCompare(n, sc, ctx)

	case *frontend.BoolOp:
		return lowerBoolOp(n, sc, ctx)

	case *frontend.Call:
		return lowerCall(n, sc, ctx)

	case *frontend.UnaryOp:
		// Only `not` has a static form here. Negation and bitwise invert are
		// arithmetic the seed does not carry, and unary plus is a no-op the
		// frontier can fold; each stays boxed until its own slice proves it.
		if n.Op != frontend.UnaryNot {
			return nil, emit.Repr{}, unsupported("unary operator %v", n.Op)
		}
		x, xr, err := lowerExpr(n.X, sc, ctx)
		if err != nil {
			return nil, emit.Repr{}, err
		}
		if xr.Scalar != emit.SBool {
			return nil, emit.Repr{}, unsupported("not needs a bool operand, got %s", xr.Scalar)
		}
		return emit.Not{X: x}, boolReprIR(), nil
	}
	return nil, emit.Repr{}, unsupported("expression %T", e)
}

// lowerCompare lowers a comparison, chained or not. Python expands `a < b < c`
// into `a < b and b < c`, so the bridge builds one emit.Cmp per adjacent pair
// and joins them with emit.And left to right, reproducing the conjunction the
// language defines. Each operand is a pure scalar in this subset, so reusing the
// middle term as both the right of one pair and the left of the next is
// evaluation-order-safe; the single-evaluation temp the frontier needs for a
// side-effecting middle term is a later slice, not this one.
func lowerCompare(n *frontend.Compare, sc scope, ctx lowerCtx) (emit.Expr, emit.Repr, error) {
	// In a chain, every term but the first and last is an operand of two adjacent
	// pairs, so the expansion reads it twice. Python evaluates it once, so a term
	// that reads twice must be single-evaluation-safe: a bare name or literal reads
	// to the identical value with no side effect and no recomputed guard, so it is
	// safe, but a computed term (arithmetic, a call) would evaluate twice, which
	// duplicates its work and any guard it carries. Rather than emit that double
	// evaluation, the chain with a computed middle term stays boxed, where the boxed
	// tier binds the term to a temp and evaluates it once. A plain (unchained)
	// comparison has no reused operand, so nothing here restricts it.
	for i := 0; i < len(n.Ops)-1; i++ {
		if !singleEvalSafe(n.Rights[i]) {
			return nil, emit.Repr{}, unsupported("a chained comparison reuses a computed middle term that would evaluate twice; kept boxed")
		}
	}
	left, leftR, err := lowerExpr(n.Left, sc, ctx)
	if err != nil {
		return nil, emit.Repr{}, err
	}
	var acc emit.Expr
	for i, k := range n.Ops {
		op, ok := cmpOp(k)
		if !ok {
			return nil, emit.Repr{}, unsupported("comparison operator %v", k)
		}
		right, rightR, err := lowerExpr(n.Rights[i], sc, ctx)
		if err != nil {
			return nil, emit.Repr{}, err
		}
		if err := cmpOperands(op, leftR, rightR); err != nil {
			return nil, emit.Repr{}, err
		}
		cmp := emit.Cmp{Op: op, L: left, R: right}
		if acc == nil {
			acc = cmp
		} else {
			acc = emit.And{L: acc, R: cmp}
		}
		left, leftR = right, rightR
	}
	return acc, boolReprIR(), nil
}

// lowerBoolOp folds `a and b and c` (or the or-chain) left into nested emit
// connectives. Two proven bool operands lower to Go's own && and || with a bool
// result. Python's value-returning `x or y` on two non-bool operands returns an
// operand rather than a coerced bool: when the whole chain shares one non-bool
// scalar the result is that scalar, selected by truthiness through a runtime
// helper. A mixed chain (say a bool with an int, or an int with a string) has no
// single static type, so it keeps the unit boxed rather than force a type.
func lowerBoolOp(n *frontend.BoolOp, sc scope, ctx lowerCtx) (emit.Expr, emit.Repr, error) {
	if len(n.Values) < 2 {
		return nil, emit.Repr{}, unsupported("boolean connective with fewer than two operands")
	}
	acc, accR, err := lowerExpr(n.Values[0], sc, ctx)
	if err != nil {
		return nil, emit.Repr{}, err
	}
	// The chain has one static value type only when every operand shares a scalar the
	// static tier can hold: bool folds to &&/||, and int, float, or string folds to
	// the value-select helper. Any other operand (or a mix) keeps the unit boxed.
	if !connScalar(accR.Scalar) {
		return nil, emit.Repr{}, unsupported("%s needs same-typed scalar operands, got %s", boolName(n.Kind), accR.Scalar)
	}
	for _, v := range n.Values[1:] {
		r, rr, err := lowerExpr(v, sc, ctx)
		if err != nil {
			return nil, emit.Repr{}, err
		}
		if rr.Scalar != accR.Scalar {
			return nil, emit.Repr{}, unsupported("%s needs same-typed scalar operands, got %s and %s", boolName(n.Kind), accR.Scalar, rr.Scalar)
		}
		// Every operand past the first is evaluated only when the ones before it do
		// not decide the result, so an operand that can raise (a division's zero
		// check) cannot short-circuit safely once its guard hoists to the statement
		// boundary. Keep the unit boxed rather than raise where Python would not; this
		// mirrors emit's own refusal so the tier decision agrees with what emit emits.
		if emit.HasRaisingGuard(r) {
			return nil, emit.Repr{}, unsupported("a %s operand that can raise cannot short-circuit safely in the static tier", boolName(n.Kind))
		}
		if n.Kind == frontend.BoolAnd {
			acc = emit.And{L: acc, R: r}
		} else {
			acc = emit.Or{L: acc, R: r}
		}
	}
	// A bool chain is a bool; a non-bool chain returns the operand scalar it shares.
	if accR.Scalar == emit.SBool {
		return acc, boolReprIR(), nil
	}
	return acc, accR, nil
}

// lowerCall lowers a call to another static unit into a direct, monomorphic Go
// call. The bridge only reaches a static call through a resolver that names the
// callee's emitted Go function and its unboxed signature; without one (a nil
// resolver, or a name the resolver does not know) the call stays boxed, exactly
// the refusal a caller with no known-static callees wants. Only the plain
// positional call shape lowers: a keyword, star, or double-star argument, or a
// callee that is anything but a bare name, has no monomorphic Go form here and is
// refused. Each argument's proven scalar must match the callee's parameter
// representation, since a mismatched shape would hand emit a call it cannot type;
// on a match the call lowers to emit.Call, which prints the direct invocation and
// threads the D14 error to the caller's own return.
func lowerCall(n *frontend.Call, sc scope, ctx lowerCtx) (emit.Expr, emit.Repr, error) {
	name, ok := n.Fn.(*frontend.Name)
	if !ok {
		return nil, emit.Repr{}, unsupported("only a call to a bare name has a static form, not %T", n.Fn)
	}
	if ctx.resolve == nil {
		return nil, emit.Repr{}, unsupported("call to %s: no static callee resolver", name.Id)
	}
	callee, ok := ctx.resolve(name.Id)
	if !ok {
		return nil, emit.Repr{}, unsupported("call to %s: not a known static callee", name.Id)
	}
	for _, a := range n.Args {
		if a.Name != "" {
			return nil, emit.Repr{}, unsupported("call to %s uses a keyword argument, which has no static form", name.Id)
		}
		if a.Star != 0 {
			return nil, emit.Repr{}, unsupported("call to %s uses a star argument, which has no static form", name.Id)
		}
	}
	if len(n.Args) != len(callee.Params) {
		return nil, emit.Repr{}, unsupported("call to %s passes %d arguments, the static callee takes %d", name.Id, len(n.Args), len(callee.Params))
	}
	args := make([]emit.Expr, len(n.Args))
	for i, a := range n.Args {
		v, r, err := lowerExpr(a.Value, sc, ctx)
		if err != nil {
			return nil, emit.Repr{}, err
		}
		if r.Scalar != callee.Params[i].Scalar {
			return nil, emit.Repr{}, unsupported("call to %s argument %d is a %s, the static callee takes a %s", name.Id, i, r.Scalar, callee.Params[i].Scalar)
		}
		args[i] = v
	}
	return emit.Call{Name: callee.GoName, Args: args, Ret: callee.Ret}, callee.Ret, nil
}

// connScalar reports whether a scalar has a static value-connective form: bool
// through Go's &&/|| and int, float, or string through the value-select helpers.
func connScalar(s emit.Scalar) bool {
	return s == emit.SBool || s == emit.SInt || s == emit.SFloat || s == emit.SStr
}

// singleEvalSafe reports whether an expression can be read more than once with no
// change in value and no side effect, so a chained comparison may reuse it as a
// shared middle term without a temp. A bare name reads a binding and a literal is
// a constant, both side-effect-free and stable; anything computed (arithmetic, a
// comparison, a call) is not, since re-reading it re-does the work and any guard.
func singleEvalSafe(e frontend.Expr) bool {
	switch e.(type) {
	case *frontend.Name, *frontend.IntLit, *frontend.FloatLit, *frontend.BoolLit, *frontend.StrLit:
		return true
	}
	return false
}

// cmpOp maps the frontend's comparison operators to emit's six. Membership
// (`in`, `not in`) and identity (`is`, `is not`) have no scalar form: membership
// is a container operation and identity is a CPython object-cache detail, not
// value equality, so both are refused and stay boxed (04/05 refusal items).
func cmpOp(k frontend.CmpKind) (emit.CmpOp, bool) {
	switch k {
	case frontend.CmpEq:
		return emit.CmpEq, true
	case frontend.CmpNe:
		return emit.CmpNe, true
	case frontend.CmpLt:
		return emit.CmpLt, true
	case frontend.CmpLe:
		return emit.CmpLe, true
	case frontend.CmpGt:
		return emit.CmpGt, true
	case frontend.CmpGe:
		return emit.CmpGe, true
	}
	return 0, false
}

// cmpOperands reproduces emit's own comparison operand rules so the bridge
// refuses a pairing emit would reject rather than handing it a node that fails
// at emission: numbers compare (a mixed int-and-float pair coerces to float),
// strings compare, and bools take equality only, since ordering bools has no
// static form.
func cmpOperands(op emit.CmpOp, l, r emit.Repr) error {
	switch {
	case arith(l) && arith(r):
		return nil
	case l.Scalar == emit.SStr && r.Scalar == emit.SStr:
		return nil
	case l.Scalar == emit.SBool && r.Scalar == emit.SBool && !cmpOrdered(op):
		return nil
	}
	return unsupported("%s does not compare %s and %s", op, l.Scalar, r.Scalar)
}

// cmpOrdered reports whether an operator is an ordering comparison, the ones
// bool operands may not take. It mirrors emit's own ordered check, which is
// unexported.
func cmpOrdered(op emit.CmpOp) bool { return op != emit.CmpEq && op != emit.CmpNe }

// boolName spells a connective for a diagnostic.
func boolName(k frontend.BoolKind) string {
	if k == frontend.BoolAnd {
		return "and"
	}
	return "or"
}

// boolReprIR is the bool representation the comparison and connective nodes
// produce, spelled here to match emit's boolRepr without reaching across the
// package boundary.
func boolReprIR() emit.Repr { return emit.Repr{Go: "bool", Scalar: emit.SBool, Total: true} }

// binOp maps the frontend's arithmetic operators to the ones the scalar tier
// lowers: the four core operators plus integer floor division and modulo. Power,
// the bitwise operators, and matrix multiply are not in this seed.
func binOp(k frontend.BinKind) (emit.Op, bool) {
	switch k {
	case frontend.BinAdd:
		return emit.OpAdd, true
	case frontend.BinSub:
		return emit.OpSub, true
	case frontend.BinMul:
		return emit.OpMul, true
	case frontend.BinDiv:
		return emit.OpDiv, true
	case frontend.BinFloorDiv:
		return emit.OpFloorDiv, true
	case frontend.BinMod:
		return emit.OpMod, true
	}
	return 0, false
}

// binResult reproduces emit's own operand rules so the bridge tracks the same
// representation emit will compute when it lowers the node: string concatenation
// stays string, true division is always float, a float operand promotes the
// result to float, and two ints stay int. Any other operand pairing is refused
// here rather than handed to emit to fail on.
func binResult(op emit.Op, l, r emit.Repr) (emit.Repr, error) {
	if l.Scalar == emit.SStr || r.Scalar == emit.SStr {
		if op != emit.OpAdd || l.Scalar != emit.SStr || r.Scalar != emit.SStr {
			return emit.Repr{}, unsupported("%s on strings", op)
		}
		return emit.Repr{Go: "string", Scalar: emit.SStr, Total: true}, nil
	}
	if !numeric(l) || !numeric(r) {
		return emit.Repr{}, unsupported("%s needs numeric operands, got %s and %s", op, l.Scalar, r.Scalar)
	}
	// Floor division lowers only on two ints at M4: the floored-int result carries
	// the overflow guard and the zero-division check. A float operand would need the
	// float floor form (math.Floor of the quotient), which stays boxed for now, so it
	// is refused here rather than lowered to the wrong shape.
	if op == emit.OpFloorDiv {
		if l.Scalar == emit.SFloat || r.Scalar == emit.SFloat {
			return emit.Repr{}, unsupported("// on a float operand stays boxed at M4")
		}
		return emit.Repr{Go: "int64", Scalar: emit.SInt}, nil
	}
	// Modulo lowers only on two ints at M4, the same as floor division: the floored
	// remainder is an int carrying the divisor's sign, with a zero-division check and
	// no overflow. A float operand would need the float modulo (math.Mod with a floor
	// correction), which stays boxed for now, so it is refused here.
	if op == emit.OpMod {
		if l.Scalar == emit.SFloat || r.Scalar == emit.SFloat {
			return emit.Repr{}, unsupported("%% on a float operand stays boxed at M4")
		}
		return emit.Repr{Go: "int64", Scalar: emit.SInt}, nil
	}
	if op == emit.OpDiv || l.Scalar == emit.SFloat || r.Scalar == emit.SFloat {
		return emit.Repr{Go: "float64", Scalar: emit.SFloat, Total: true}, nil
	}
	// Two ints, or a bool with an int, or two bools: bool is a subtype of int, so the
	// result is a plain int (`True + True` is `2`).
	return emit.Repr{Go: "int64", Scalar: emit.SInt}, nil
}

// arith reports whether a representation is an int or float, the only operands a
// scalar comparison accepts.
func arith(r emit.Repr) bool { return r.Scalar == emit.SInt || r.Scalar == emit.SFloat }

// numeric reports whether a representation may be an arithmetic operand. It
// mirrors emit's own rule: int and float are numeric, and bool joins them because
// Python's bool is a subtype of int, so `True + 1.0` is `2.0`.
func numeric(r emit.Repr) bool {
	return r.Scalar == emit.SInt || r.Scalar == emit.SFloat || r.Scalar == emit.SBool
}
