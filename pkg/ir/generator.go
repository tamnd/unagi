package ir

import (
	"github.com/tamnd/unagi/pkg/emit"
	"github.com/tamnd/unagi/pkg/frontend"
)

// A generator def does not lower to an emit.Func; it lowers to an emit.Generator,
// the state machine pkg/emit/gen.go renders as a struct plus a Next method that
// switches on a discriminant. This file is the bridge for that shape: it splits a
// straight-line generator body into one segment per yield, lifts the parameters
// the segments read across a suspension onto the saved-field set, and hands emit
// the segments in source order.
//
// This first slice lowers the flat shape only: a body that is a sequence of
// `yield <expr>` statements with optional within-segment locals, over scalar
// parameters. A loop or an `if` around a yield, a local that outlives its
// segment, a `yield from`, a bare or valued `return`, or a yield used as a
// sub-expression all refuse here, so the unit stays boxed until the loop and
// guarded-segment shapes land. The refusal is the R5-safe outcome: an
// unrecognized generator runs on the boxed goroutine tier, byte-identical to
// python3.14, never a half-lowered machine.

// IsGenerator reports whether a def is a generator, which is true when a yield
// appears anywhere in its body. The parser only admits a yield inside a function,
// so a positive result here means the def suspends and must lower through the
// generator bridge rather than the scalar-function bridge.
func IsGenerator(fn *frontend.FuncDef) bool {
	return containsYield(fn.Body)
}

func containsYield(stmts []frontend.Stmt) bool {
	for _, s := range stmts {
		switch n := s.(type) {
		case *frontend.ExprStmt:
			if yieldIn(n.X) {
				return true
			}
		case *frontend.Assign:
			if yieldIn(n.Value) {
				return true
			}
		case *frontend.AnnAssign:
			if yieldIn(n.Value) {
				return true
			}
		case *frontend.Return:
			if yieldIn(n.Value) {
				return true
			}
		case *frontend.If:
			if yieldIn(n.Cond) || containsYield(n.Body) || containsYield(n.Else) {
				return true
			}
		case *frontend.While:
			if yieldIn(n.Cond) || containsYield(n.Body) || containsYield(n.Else) {
				return true
			}
		case *frontend.For:
			if yieldIn(n.Iter) || containsYield(n.Body) || containsYield(n.Else) {
				return true
			}
		}
	}
	return false
}

// yieldIn reports whether an expression tree contains a yield. A yield can nest
// inside another expression (`x + (yield)`), so the walk descends the scalar
// operand positions; any position it does not descend cannot host a yield the
// generator bridge would lower, so missing one only keeps a unit boxed.
func yieldIn(e frontend.Expr) bool {
	switch n := e.(type) {
	case *frontend.Yield:
		return true
	case *frontend.BinOp:
		return yieldIn(n.Left) || yieldIn(n.Right)
	case *frontend.UnaryOp:
		return yieldIn(n.X)
	case *frontend.BoolOp:
		for _, v := range n.Values {
			if yieldIn(v) {
				return true
			}
		}
	case *frontend.Compare:
		if yieldIn(n.Left) {
			return true
		}
		for _, r := range n.Rights {
			if yieldIn(r) {
				return true
			}
		}
	}
	return false
}

// LowerGenerator translates a straight-line generator def into the emit.Generator
// state machine, or reports an unsupported error the caller treats as "keep this
// unit boxed". On success the returned Generator prints, through
// emit.EmitGenerator, to the unboxed struct-and-switch the hand-built models
// produce.
func LowerGenerator(fn *frontend.FuncDef) (emit.Generator, error) {
	if fn.Async {
		return emit.Generator{}, unsupported("async generator %s", fn.Name)
	}
	if len(fn.Decorators) != 0 {
		return emit.Generator{}, unsupported("decorated generator %s", fn.Name)
	}
	if !containsYield(fn.Body) {
		return emit.Generator{}, unsupported("%s is not a generator", fn.Name)
	}

	sc := scope{}
	params := make([]emit.Param, 0, len(fn.Params))
	isParam := map[string]bool{}
	for _, p := range fn.Params {
		if p.Kind != frontend.ParamPlain && p.Kind != frontend.ParamPosOnly {
			return emit.Generator{}, unsupported("parameter %s is not a plain positional parameter", p.Name)
		}
		if p.Default != nil {
			return emit.Generator{}, unsupported("parameter %s has a default", p.Name)
		}
		if p.Annotation == nil {
			return emit.Generator{}, unsupported("parameter %s is unannotated", p.Name)
		}
		r, ok := annotationRepr(p.Annotation)
		if !ok {
			return emit.Generator{}, unsupported("parameter %s has a non-scalar annotation", p.Name)
		}
		params = append(params, emit.Param{Name: p.Name, Repr: r})
		sc[p.Name] = r
		isParam[p.Name] = true
	}

	reads := map[string]bool{}
	loadedNames(fn.Body, reads)
	ctx := lowerCtx{reads: reads}

	referenced := map[string]bool{}
	var segs []emit.Segment
	var pre []emit.Stmt
	curLocals := map[string]bool{}
	var elem *emit.Repr

	for _, s := range fn.Body {
		switch n := s.(type) {
		case *frontend.ExprStmt:
			y, ok := n.X.(*frontend.Yield)
			if !ok {
				return emit.Generator{}, unsupported("a bare expression statement in a generator has no static form")
			}
			if y.From {
				return emit.Generator{}, unsupported("yield from has no static form yet")
			}
			if y.Value == nil {
				return emit.Generator{}, unsupported("a bare yield has no scalar value")
			}
			v, r, err := lowerExpr(y.Value, sc, ctx)
			if err != nil {
				return emit.Generator{}, err
			}
			v, err = recvify(v, isParam, curLocals, referenced)
			if err != nil {
				return emit.Generator{}, err
			}
			if elem == nil {
				rr := r
				elem = &rr
			} else if elem.Scalar != r.Scalar {
				return emit.Generator{}, unsupported("%s yields %s in one segment and %s in another", fn.Name, elem.Scalar, r.Scalar)
			}
			segs = append(segs, emit.Segment{Pre: pre, Yield: v})
			pre = nil
			curLocals = map[string]bool{}

		case *frontend.Assign:
			if len(n.Targets) != 1 {
				return emit.Generator{}, unsupported("a chained or tuple assignment in a generator has no static form")
			}
			name, ok := n.Targets[0].(*frontend.Name)
			if !ok {
				return emit.Generator{}, unsupported("a generator assignment target is not a plain name")
			}
			if isParam[name.Id] {
				return emit.Generator{}, unsupported("a generator local %s shadows a parameter", name.Id)
			}
			if yieldIn(n.Value) {
				return emit.Generator{}, unsupported("a yield bound to a name has no static form yet")
			}
			v, r, err := lowerExpr(n.Value, sc, ctx)
			if err != nil {
				return emit.Generator{}, err
			}
			v, err = recvify(v, isParam, curLocals, referenced)
			if err != nil {
				return emit.Generator{}, err
			}
			sc[name.Id] = r
			curLocals[name.Id] = true
			pre = append(pre, emit.Define{Name: name.Id, Value: v})

		default:
			return emit.Generator{}, unsupported("statement %T has no static generator form yet", s)
		}
	}

	if len(segs) == 0 {
		return emit.Generator{}, unsupported("%s has no yield the tier can lower", fn.Name)
	}
	if len(pre) != 0 {
		return emit.Generator{}, unsupported("%s runs statements after its last yield, which needs the trailer form", fn.Name)
	}

	fields := make([]emit.GenField, 0, len(params))
	for _, p := range params {
		if referenced[p.Name] {
			fields = append(fields, emit.GenField{Name: p.Name, Repr: p.Repr})
		}
	}
	return emit.Generator{Name: fn.Name, Elem: *elem, Fields: fields, Segments: segs}, nil
}

// recvify rewrites a lowered expression so a reference to a saved field (a
// parameter) becomes g.<name>, the emit.Recv node, while a within-segment local
// stays a plain Var. It records each parameter it rewrites into referenced, which
// is how the caller learns the cross-yield live set. A Var that is neither a
// parameter nor a current-segment local, a local from an earlier segment that
// outlived it or a free name the generator subset does not carry, refuses: the
// resumable frame this slice builds saves only the parameters, so anything else
// crossing a suspension has no field to live in and the unit stays boxed.
func recvify(e emit.Expr, isParam, curLocals, referenced map[string]bool) (emit.Expr, error) {
	switch n := e.(type) {
	case emit.Var:
		if isParam[n.Name] {
			referenced[n.Name] = true
			return emit.Recv{Name: n.Name, Repr: n.Repr}, nil
		}
		if curLocals[n.Name] {
			return n, nil
		}
		return nil, unsupported("generator reads %s, which does not live in the resumable frame yet", n.Name)
	case emit.Int, emit.Float, emit.Bool, emit.Str:
		return n, nil
	case emit.Bin:
		l, err := recvify(n.L, isParam, curLocals, referenced)
		if err != nil {
			return nil, err
		}
		r, err := recvify(n.R, isParam, curLocals, referenced)
		if err != nil {
			return nil, err
		}
		return emit.Bin{Op: n.Op, L: l, R: r}, nil
	case emit.Cmp:
		l, err := recvify(n.L, isParam, curLocals, referenced)
		if err != nil {
			return nil, err
		}
		r, err := recvify(n.R, isParam, curLocals, referenced)
		if err != nil {
			return nil, err
		}
		return emit.Cmp{Op: n.Op, L: l, R: r}, nil
	case emit.And:
		l, err := recvify(n.L, isParam, curLocals, referenced)
		if err != nil {
			return nil, err
		}
		r, err := recvify(n.R, isParam, curLocals, referenced)
		if err != nil {
			return nil, err
		}
		return emit.And{L: l, R: r}, nil
	case emit.Or:
		l, err := recvify(n.L, isParam, curLocals, referenced)
		if err != nil {
			return nil, err
		}
		r, err := recvify(n.R, isParam, curLocals, referenced)
		if err != nil {
			return nil, err
		}
		return emit.Or{L: l, R: r}, nil
	case emit.Not:
		x, err := recvify(n.X, isParam, curLocals, referenced)
		if err != nil {
			return nil, err
		}
		return emit.Not{X: x}, nil
	default:
		return nil, unsupported("generator yields an expression shape the resumable frame does not carry yet")
	}
}
