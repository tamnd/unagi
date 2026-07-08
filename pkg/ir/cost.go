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
	case emit.AddAssign:
		// An accumulating += is one arithmetic operation, and on an int target it
		// carries the same overflow guard a written-out add would.
		costExpr(n.Value, c)
		c.UnboxedOps++
		if n.Repr.Scalar == emit.SInt {
			c.EntryGuards++
		}
	case emit.Return:
		costExpr(n.Value, c)
	}
}

// costExpr adds one expression's operations to the running census. Only a binary
// operation carries a cost; a variable read or a literal is free, matching the
// cost model, which charges operations, not operands.
func costExpr(e emit.Expr, c *Cost) {
	bin, ok := e.(emit.Bin)
	if !ok {
		return
	}
	costExpr(bin.L, c)
	costExpr(bin.R, c)
	c.UnboxedOps++
	// An int add, subtract, or multiply is the one operation this tier guards:
	// its result stays int only when both operands are int and the operator is
	// not true division, exactly the case binResult reports as an int result.
	if r, err := binResult(bin.Op, reprOf(bin.L), reprOf(bin.R)); err == nil && r.Scalar == emit.SInt {
		c.EntryGuards++
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
	}
	return emit.Repr{}
}
