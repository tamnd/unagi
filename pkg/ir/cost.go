package ir

import "github.com/tamnd/unagi/pkg/emit"

// This file measures a lowered static function the way doc 06 section 5.7's cost
// model wants it: an operation count and a guard count the partitioner scores
// against the boxed twin. It walks the emit model the bridge produced, so the
// numbers describe exactly the Go that would be emitted, not an estimate off the
// AST. The bridge only builds the scalar subset, so this walk only needs to
// understand that subset; a node it does not recognize contributes nothing.

// Cost is the operation census of one lowered function. UnboxedOps counts the
// native arithmetic and concatenation operations; EntryGuards and LoopGuards
// count the overflow guards an int operation carries, split by whether the guard
// sits at function entry or inside a loop. The bridge does not lower loops yet,
// so LoopGuards is always zero today, but the field is here so the loop case
// slots in without reshaping the profile.
type Cost struct {
	UnboxedOps  int
	EntryGuards int
	LoopGuards  int
}

// CostOf walks a lowered function and returns its operation census. It is a pure
// function of the emit model, so two runs over the same function return the same
// counts, which is what keeps the partition decision reproducible.
func CostOf(f emit.Func) Cost {
	var c Cost
	for _, s := range f.Body {
		costStmt(s, &c)
	}
	return c
}

// costStmt adds one statement's operations to the running census.
func costStmt(s emit.Stmt, c *Cost) {
	switch n := s.(type) {
	case emit.Define:
		costExpr(n.Value, c)
	case emit.Assign:
		// A rebinding charges only its value's operations; the assignment itself is a
		// register move, not an arithmetic operation, exactly as a Define is.
		costExpr(n.Value, c)
	case emit.Bind:
		// A parallel binding charges each value's operations; the binding itself is a
		// set of register moves, no arithmetic of its own.
		for _, v := range n.Values {
			costExpr(v, c)
		}
	case emit.VarDecl:
		// A join declaration carries no value and no arithmetic; it only names a local
		// and its Go type ahead of the branch that assigns it, so it costs nothing.
	case emit.AugAssign:
		// An accumulating += is one arithmetic operation, and on an int target it
		// carries the same overflow guard a written-out add would.
		costExpr(n.Value, c)
		c.UnboxedOps++
		if n.Repr.Scalar == emit.SInt {
			c.EntryGuards++
		}
	case emit.Return:
		costExpr(n.Value, c)
	case emit.Discard:
		// A bare expression statement charges its value's operations and nothing more:
		// the result is thrown away, so there is no binding, but a discarded call still
		// runs and a discarded guarded operation still guards, so the walk into the value
		// carries both. A pure discardable value (a docstring, a bare name) folds to a nil
		// value at the bridge and never reaches here.
		costExpr(n.Value, c)
	case emit.If:
		// The condition and both arms all emit their own guards, so the census walks
		// every one: a guarded int operation in a condition or a branch still carries
		// its overflow guard, and missing it would let a function that emits a guard
		// and a deopt edge pass as guard-free static, the mislabel D4 forbids. A guard
		// under an if sits at function entry, not in a loop back-edge, so it counts as
		// an entry guard the same as one in a straight-line body.
		costExpr(n.Cond, c)
		for _, s := range n.Then {
			costStmt(s, c)
		}
		for _, s := range n.Else {
			costStmt(s, c)
		}
	case emit.While:
		// A while runs its condition and body once per iteration, so every guard either
		// carries fires on the loop back-edge, not at function entry: the census walks the
		// condition and the body into a sub-cost and folds all of its guards into the
		// loop-guard bucket. The bridge refuses a guarded while at M4 (the back-edge resume
		// point is a later slice), so this bucket is zero today, but classifying it here is
		// what lets the loop deopt case slot in without reshaping the profile.
		var bc Cost
		costExpr(n.Cond, &bc)
		for _, s := range n.Body {
			costStmt(s, &bc)
		}
		c.UnboxedOps += bc.UnboxedOps
		c.LoopGuards += bc.EntryGuards + bc.LoopGuards
	case emit.ForCount:
		// A counting loop runs its body once per iteration, so a body guard fires on the
		// loop back-edge and counts as a loop guard, the same classification the while case
		// makes. The induction `i++` and the bound test carry no overflow guard of their
		// own: the bridge admits only an int64 bound, over which the int64 induction cannot
		// overflow before the test fails, so the loop header contributes nothing. The bridge
		// refuses a guarded body at M4, so this bucket is zero today.
		var bc Cost
		costExpr(n.Start, &bc)
		costExpr(n.Stop, &bc)
		for _, s := range n.Body {
			costStmt(s, &bc)
		}
		c.UnboxedOps += bc.UnboxedOps
		c.LoopGuards += bc.EntryGuards + bc.LoopGuards
	}
}

// costExpr adds one expression's operations to the running census. An arithmetic
// binary, a comparison, and a connective each count one operation; a variable
// read or a literal is free, matching the cost model, which charges operations,
// not operands. The walk recurses into every operand, so a guarded int operation
// nested inside a comparison or a connective (`a + b < c`) still contributes its
// overflow guard: missing it would let a function that actually emits a guard and
// a deopt edge pass as guard-free static, exactly the mislabel D4 forbids.
func costExpr(e emit.Expr, c *Cost) {
	switch n := e.(type) {
	case emit.Bin:
		costExpr(n.L, c)
		costExpr(n.R, c)
		c.UnboxedOps++
		// An int add, subtract, multiply, or floor division is an operation this tier
		// guards: its result stays int, and the operator can carry a value past int64,
		// so it emits an overflow guard and a deopt edge. Modulo also has an int result
		// but cannot overflow, so Overflows screens it out, and true division is float,
		// which the int-result test already excludes.
		if r, err := binResult(n.Op, reprOf(n.L), reprOf(n.R)); err == nil && r.Scalar == emit.SInt && n.Op.Overflows() {
			c.EntryGuards++
		}
	case emit.Cmp:
		costExpr(n.L, c)
		costExpr(n.R, c)
		c.UnboxedOps++
	case emit.And:
		costExpr(n.L, c)
		costExpr(n.R, c)
		c.UnboxedOps++
	case emit.Or:
		costExpr(n.L, c)
		costExpr(n.R, c)
		c.UnboxedOps++
	case emit.Not:
		costExpr(n.X, c)
		c.UnboxedOps++
	case emit.Call:
		// A direct static-to-static call is one unboxed operation, the same as any
		// native op: it threads the callee's error and carries no overflow guard of
		// its own, so it adds no guard here. Each argument is a scalar expression that
		// contributes its own operations, so the walk recurses into every one.
		for _, a := range n.Args {
			costExpr(a, c)
		}
		c.UnboxedOps++
	case emit.Index:
		// A bounds-guarded read is one unboxed operation plus the bounds guard that
		// deopts on a negative or out-of-range index, so it counts a guard the same
		// way an overflowing int op does. The base and index each contribute their own
		// operations.
		costExpr(n.Base, c)
		costExpr(n.Idx, c)
		c.UnboxedOps++
		c.EntryGuards++
	case emit.ListLit:
		// A list literal builds one slice value; each item contributes its own
		// operations. The construction carries no guard of its own.
		for _, it := range n.Items {
			costExpr(it, c)
		}
		c.UnboxedOps++
	}
}

// reprOf recovers the representation of an emit expression the bridge built, so
// costExpr can tell a guarded int operation from a total float one without a
// second inference pass. It understands only the nodes the bridge emits; any
// other node reports the zero representation, which costExpr treats as unguarded.
func reprOf(e emit.Expr) emit.Repr {
	switch n := e.(type) {
	case emit.Var:
		return n.Repr
	case emit.Int:
		return emit.Repr{Go: "int64", Scalar: emit.SInt}
	case emit.Float:
		return emit.Repr{Go: "float64", Scalar: emit.SFloat, Total: true}
	case emit.Bool:
		return emit.Repr{Go: "bool", Scalar: emit.SBool, Total: true}
	case emit.Str:
		return emit.Repr{Go: "string", Scalar: emit.SStr, Total: true}
	case emit.Bin:
		r, err := binResult(n.Op, reprOf(n.L), reprOf(n.R))
		if err != nil {
			return emit.Repr{}
		}
		return r
	case emit.Cmp, emit.Not:
		return boolReprIR()
	case emit.And:
		// A connective on two bools is a bool; the value-returning form on a shared
		// non-bool scalar returns that operand, so its result repr is the operand's.
		return reprOf(n.L)
	case emit.Or:
		return reprOf(n.L)
	case emit.Call:
		// A direct call's result is the callee's declared return representation, which
		// the bridge stamped on the node from the resolver's static signature.
		return n.Ret
	case emit.Index:
		// A read yields the base list's element representation.
		if br := reprOf(n.Base); br.Elem != nil {
			return *br.Elem
		}
		return emit.Repr{}
	case emit.ListLit:
		// A literal's representation is a slice of its proven element type.
		elem := n.Elem
		return emit.Repr{Go: "[]" + elem.Go, Total: true, Elem: &elem}
	}
	return emit.Repr{}
}
