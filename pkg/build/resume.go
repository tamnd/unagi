package build

import (
	"github.com/tamnd/unagi/pkg/emit"
	"github.com/tamnd/unagi/pkg/frontend"
)

// setLoopResume finds the counting loop in a lowered static form and marks it for
// mid-loop resume: a guard inside its body will hand off to handler carrying the
// counter and the accumulator instead of replaying from the top. The canonical
// shape has one top-level loop, so the first ForCount is the one to mark; a body
// without one leaves the from-top edge untouched.
func setLoopResume(body []emit.Stmt, handler, acc string) bool {
	for i, s := range body {
		if loop, ok := s.(emit.ForCount); ok {
			loop.Resume = &emit.ResumeInfo{Handler: handler, Carried: []string{acc}}
			body[i] = loop
			return true
		}
	}
	return false
}

// This file proves when a deopt-target counting loop can resume mid-loop instead
// of replaying from the top, and synthesizes the boxed twin that resumes it. B3a
// already made a guarded loop sound by re-running the whole unit boxed from
// function entry, which recomputes the iterations the native loop already ran; the
// mid-loop resume skips that rework by re-entering the boxed twin at the failing
// iteration with the accumulator the native loop held. It is a pure performance
// optimization on the rare failing activation, so it only fires for a shape where
// re-entering at the current iteration is provably identical to the from-top
// replay, and every other loop keeps the from-top edge that is always correct.
//
// The provable shape is the canonical single-accumulator counting loop: a
// function that initializes one integer accumulator, runs one ascending
// `for v in range(...)` loop whose body is the single guarded update of that
// accumulator, and returns. Because the accumulator is the only state the loop
// carries and its update is the guarded statement, the accumulator's value at the
// guard is exactly its value at the top of the iteration, so re-running that
// whole iteration boxed from the accumulator's guard-time value reproduces the
// from-top result while skipping the iterations already run. Any loop that mutates
// other state, runs more than one body statement, reads another outer local, or
// counts down or by a computed bound falls outside this proof and stays on the
// from-top edge.

// resumeShape is one function's proven mid-loop resume plan: the loop counter and
// the single carried accumulator the twin re-enters with, and the synthesized
// boxed twin that runs the loop tail from that state.
type resumeShape struct {
	loopVar string
	acc     string
	twin    *frontend.FuncDef
}

// resumeShapeFor proves the canonical single-accumulator counting loop and
// returns its resume plan, reporting false for every function that does not fit
// so the caller keeps the from-top deopt edge. The def is one the partitioner
// already proved static with a non-empty deopt plan, so its scalar shape is known;
// this walk adds the structural checks the resume proof needs on top.
func resumeShapeFor(d *frontend.FuncDef) (*resumeShape, bool) {
	// Split the body into the leading initializers, the one counting loop, and
	// whatever trails it. A body without exactly one top-level for loop, or with a
	// loop that is not the last real statement bar a single return, is not the
	// canonical shape.
	var leads []*frontend.Assign
	var loop *frontend.For
	var tail []frontend.Stmt
	for _, s := range d.Body {
		switch n := s.(type) {
		case *frontend.Assign:
			if loop != nil {
				tail = append(tail, s)
				continue
			}
			if len(n.Targets) != 1 {
				return nil, false
			}
			if _, ok := n.Targets[0].(*frontend.Name); !ok {
				return nil, false
			}
			leads = append(leads, n)
		case *frontend.For:
			if loop != nil {
				return nil, false // a second loop is outside the proof
			}
			loop = n
		default:
			if loop == nil {
				return nil, false // a non-assign before the loop
			}
			tail = append(tail, s)
		}
	}
	if loop == nil {
		return nil, false
	}

	loopVar, stopArg, ok := ascendingRange(loop)
	if !ok {
		return nil, false
	}
	if len(loop.Body) != 1 {
		return nil, false
	}
	acc, ok := guardedAccumulator(loop.Body[0])
	if !ok {
		return nil, false
	}
	// The accumulator must be one of the names initialized before the loop, so the
	// twin can take it as a parameter in place of that initializer.
	if !assignsName(leads, acc) {
		return nil, false
	}
	// The loop body may read only the accumulator, the loop counter, the function
	// parameters, and literal constants. Any other name is a second piece of outer
	// state the resume would drop, so it disqualifies the shape.
	params := paramNames(d)
	allowed := map[string]bool{acc: true, loopVar: true}
	for _, p := range params {
		allowed[p] = true
	}
	if !refsWithin(loop.Body[0], allowed) {
		return nil, false
	}
	// The tail is at most a single return of the accumulator or a parameter, the
	// names the twin still has in scope after dropping the other initializers.
	if !tailWithin(tail, allowed) {
		return nil, false
	}

	twin := synthResumeTwin(d, loopVar, acc, stopArg, loop.Body[0], tail)
	return &resumeShape{loopVar: loopVar, acc: acc, twin: twin}, true
}

// ascendingRange reports the loop variable and the stop expression of an ascending
// counting loop `for v in range(...)`, matching the bridge's counting-loop rules:
// a plain-name target, a range call of one to three positional arguments, an
// ascending step, and a stop that is a literal or a name the twin can name. It
// reports false for any other iterable, a descending or non-unit step, a computed
// bound, or a tuple target.
func ascendingRange(loop *frontend.For) (loopVar string, stop frontend.Expr, ok bool) {
	if len(loop.Else) > 0 {
		return "", nil, false
	}
	name, ok := loop.Target.(*frontend.Name)
	if !ok {
		return "", nil, false
	}
	call, ok := loop.Iter.(*frontend.Call)
	if !ok {
		return "", nil, false
	}
	fn, ok := call.Fn.(*frontend.Name)
	if !ok || fn.Id != "range" {
		return "", nil, false
	}
	for _, a := range call.Args {
		if a.Star != 0 || a.Name != "" {
			return "", nil, false
		}
	}
	var stopArg, stepArg frontend.Expr
	switch len(call.Args) {
	case 1:
		stopArg = call.Args[0].Value
	case 2:
		stopArg = call.Args[1].Value
	case 3:
		stopArg = call.Args[1].Value
		stepArg = call.Args[2].Value
	default:
		return "", nil, false
	}
	// Only an ascending unit step keeps the counter monotonic and the resume start
	// equal to the failing iteration; a descending or wider step is the bridge's
	// boxed case and never reaches a static form to resume.
	if stepArg != nil && !isIntLit(stepArg, "1") {
		return "", nil, false
	}
	if !simpleBound(stopArg) {
		return "", nil, false
	}
	return name.Id, stopArg, true
}

// guardedAccumulator reports the accumulator name of a single overflow-guarded
// update statement, `acc = acc <op> e` or `acc <op>= e` for an integer-overflowing
// op. A statement that is not one of those two forms, or whose op cannot overflow
// (so it opens no deopt edge), reports false.
func guardedAccumulator(s frontend.Stmt) (string, bool) {
	switch n := s.(type) {
	case *frontend.AugAssign:
		name, ok := n.Target.(*frontend.Name)
		if !ok || !overflowingOp(n.Op) {
			return "", false
		}
		return name.Id, true
	case *frontend.Assign:
		if len(n.Targets) != 1 {
			return "", false
		}
		name, ok := n.Targets[0].(*frontend.Name)
		if !ok {
			return "", false
		}
		bin, ok := n.Value.(*frontend.BinOp)
		if !ok || !overflowingOp(bin.Op) {
			return "", false
		}
		return name.Id, true
	}
	return "", false
}

// overflowingOp reports whether a binary op can carry an int result past int64, so
// its static lowering opens an overflow deopt edge. These are exactly the ops the
// emitter guards: add, subtract, multiply, floor division, power, and left shift.
func overflowingOp(op frontend.BinKind) bool {
	switch op {
	case frontend.BinAdd, frontend.BinSub, frontend.BinMul,
		frontend.BinFloorDiv, frontend.BinPow, frontend.BinLShift:
		return true
	}
	return false
}

// synthResumeTwin builds the boxed twin that resumes the loop at the failing
// iteration. It is the original function with the accumulator and the loop counter
// promoted to leading parameters, the initializers dropped (their names now arrive
// as parameters), and the loop restarted from the counter: `for v in range(v,
// stop)`. Running it from the counter's current value with the accumulator's
// guard-time value reproduces the from-top result over the remaining iterations.
func synthResumeTwin(d *frontend.FuncDef, loopVar, acc string, stop frontend.Expr, body frontend.Stmt, tail []frontend.Stmt) *frontend.FuncDef {
	params := make([]frontend.Param, 0, len(d.Params)+2)
	params = append(params,
		frontend.Param{Name: loopVar, Kind: frontend.ParamPlain},
		frontend.Param{Name: acc, Kind: frontend.ParamPlain},
	)
	params = append(params, d.Params...)

	loop := &frontend.For{
		Target: &frontend.Name{Id: loopVar},
		Iter: &frontend.Call{
			Fn: &frontend.Name{Id: "range"},
			Args: []frontend.Arg{
				{Value: &frontend.Name{Id: loopVar}},
				{Value: stop},
			},
		},
		Body: []frontend.Stmt{body},
	}
	twinBody := make([]frontend.Stmt, 0, 1+len(tail))
	twinBody = append(twinBody, loop)
	twinBody = append(twinBody, tail...)
	return &frontend.FuncDef{Name: d.Name, Params: params, Body: twinBody}
}

// assignsName reports whether one of the leading initializers binds name.
func assignsName(leads []*frontend.Assign, name string) bool {
	for _, a := range leads {
		if n, ok := a.Targets[0].(*frontend.Name); ok && n.Id == name {
			return true
		}
	}
	return false
}

// paramNames returns the function's parameter names.
func paramNames(d *frontend.FuncDef) []string {
	out := make([]string, len(d.Params))
	for i, p := range d.Params {
		out[i] = p.Name
	}
	return out
}

// simpleBound reports whether a stop bound is a form the twin can restate: an
// integer literal or a plain name (a parameter). A computed bound would have to be
// re-evaluated in the twin from state the twin may not hold, so it is excluded.
func simpleBound(e frontend.Expr) bool {
	switch e.(type) {
	case *frontend.IntLit, *frontend.Name:
		return true
	}
	return false
}

// isIntLit reports whether e is the integer literal with the given normalized
// text, optionally written with a single leading unary plus.
func isIntLit(e frontend.Expr, text string) bool {
	if u, ok := e.(*frontend.UnaryOp); ok && u.Op == frontend.UnaryPos {
		e = u.X
	}
	lit, ok := e.(*frontend.IntLit)
	return ok && lit.Text == text
}

// tailWithin reports whether the loop tail is empty or a single return whose
// value reads only the allowed names, the state the twin still holds.
func tailWithin(tail []frontend.Stmt, allowed map[string]bool) bool {
	if len(tail) == 0 {
		return true
	}
	if len(tail) != 1 {
		return false
	}
	ret, ok := tail[0].(*frontend.Return)
	if !ok {
		return false
	}
	if ret.Value == nil {
		return true
	}
	return refsExprWithin(ret.Value, allowed)
}

// refsWithin reports whether every name a statement reads is in allowed.
func refsWithin(s frontend.Stmt, allowed map[string]bool) bool {
	switch n := s.(type) {
	case *frontend.AugAssign:
		return refsExprWithin(n.Value, allowed)
	case *frontend.Assign:
		return refsExprWithin(n.Value, allowed)
	}
	return false
}

// refsExprWithin reports whether every Name an expression reads is in allowed. It
// walks the scalar expression forms the bridge lowers; a form it does not know
// (an attribute, a subscript, a call to anything but a bare name) is conservatively
// rejected, since an unrecognized read could be outer state the resume drops.
func refsExprWithin(e frontend.Expr, allowed map[string]bool) bool {
	switch n := e.(type) {
	case *frontend.Name:
		return allowed[n.Id]
	case *frontend.IntLit, *frontend.FloatLit, *frontend.BoolLit, *frontend.StrLit:
		return true
	case *frontend.UnaryOp:
		return refsExprWithin(n.X, allowed)
	case *frontend.BinOp:
		return refsExprWithin(n.Left, allowed) && refsExprWithin(n.Right, allowed)
	case *frontend.BoolOp:
		for _, v := range n.Values {
			if !refsExprWithin(v, allowed) {
				return false
			}
		}
		return true
	case *frontend.Compare:
		if !refsExprWithin(n.Left, allowed) {
			return false
		}
		for _, r := range n.Rights {
			if !refsExprWithin(r, allowed) {
				return false
			}
		}
		return true
	}
	return false
}
