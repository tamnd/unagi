package emit

import (
	"fmt"
	"go/ast"
	"go/token"
)

// This file emits a static generator as a state-machine struct, the D16 shape doc
// 06 section 8.7 requires. A boxed generator runs its body in a goroutine behind a
// yielder handle; a static generator instead carries an explicit discriminant and
// resumes through a switch, so there is no per-generator goroutine and no channel
// handoff. The struct holds the discriminant and every value live across a
// suspension as a field; Next switches on the discriminant, runs the segment up to
// the next yield, advances the discriminant, and returns the yielded value with
// done false, or returns the zero value with done true when the machine runs off
// the end.
//
// Each yield is a resume point. Section 8.7 pins a transfer table from every
// resume point to the boxed resumable-frame representation, materialized lazily on
// the first guard failure after a resume; this file assigns the discriminant
// states those transfer tables key on and keeps them in source order, so the
// mapping is stable across builds. The general transform from arbitrary control
// flow into segments is a doc 07 lowering that lands with the IR; this file emits
// the machine the transform targets and holds its invariants.

// genRecv is the receiver name every static generator method binds.
const genRecv = "g"

// stateField is the discriminant field name.
const stateField = "state"

// GenField is one value the machine saves across suspensions: a parameter or a
// local live at a yield, stored on the struct and read back through the receiver.
type GenField struct {
	Name string
	Repr Repr
}

// Segment is one run of the machine: statements that execute up to a yield, then
// the yielded value. The statements and the value read saved state through Recv.
// A segment with Loop set is a counting loop turned inside out: it re-enters its
// own state on each call until the counter reaches the bound. A segment with Guard
// set is a yield inside an `if`: it yields only when the guard holds and otherwise
// hands the call on to the following segment.
type Segment struct {
	Pre   []Stmt
	Yield Expr
	Loop  *LoopYield
	Guard Expr
}

// LoopYield turns a `for i in range(bound): ...; yield e` into a self-resuming
// segment, the loop-carried case doc 06 section 8.7 requires. Induction names the
// saved int field that holds the loop counter i: it starts at the field's zero
// value, the machine reads it to build each yield, then increments it. Bound is
// the range's exclusive upper bound. While i < bound the segment yields and stays
// in its own state, so the next call re-enters the loop body at the saved counter;
// once i reaches bound the machine advances to the following state. The counter is
// a field, not a local, precisely because it is live across the suspension.
type LoopYield struct {
	Induction string
	Bound     Expr
}

// Recv is a reference to a saved field, g.Name, the way a generator segment names
// a value that outlived its last suspension.
type Recv struct {
	Name string
	Repr Repr
}

func (Recv) isExpr() {}

// Generator is a static generator to emit: the struct type name, the element
// representation Next yields, the saved fields, the yielding segments in source
// order, and any trailing statements that run after the last yield before the
// machine reports done.
type Generator struct {
	Name     string
	Elem     Repr
	Fields   []GenField
	Segments []Segment
	Trailer  []Stmt
}

// EmitGenerator lowers a static generator to gofmt-clean Go: the state struct
// followed by its Next method. It returns the two declarations joined by a blank
// line, or a lowering error if a segment does not lower.
func EmitGenerator(gen Generator) (string, error) {
	typ, err := Print(genStruct(gen))
	if err != nil {
		return "", err
	}
	b := &Builder{fn: gen.Name, ret: gen.Elem}
	next, err := genNext(b, gen)
	if err != nil {
		return "", err
	}
	text, err := Print(next)
	if err != nil {
		return "", err
	}
	return typ + "\n\n" + text, nil
}

// genStruct builds the state-machine struct: the int discriminant first, then the
// saved fields in source order, so the layout is deterministic.
func genStruct(gen Generator) *ast.GenDecl {
	fields := []*ast.Field{field(ident("int"), stateField)}
	for _, f := range gen.Fields {
		fields = append(fields, field(f.Repr.goType(), f.Name))
	}
	return &ast.GenDecl{
		Tok: token.TYPE,
		Specs: []ast.Spec{&ast.TypeSpec{
			Name: ident(gen.Name),
			Type: &ast.StructType{Fields: fieldList(fields...)},
		}},
	}
}

// genNext builds the Next method: a switch on the discriminant with one case per
// segment. A segment's case runs its pre-statements, advances the discriminant to
// the next state, and returns the yielded value with done false. After the switch,
// the trailing statements run once and the machine returns the zero value with
// done true; a machine that has run off the end re-enters no case and falls
// straight to that done return.
func genNext(b *Builder, gen Generator) (*ast.FuncDecl, error) {
	var cases []ast.Stmt
	for i, seg := range gen.Segments {
		var body []ast.Stmt
		var err error
		switch {
		case seg.Loop != nil:
			body, err = loopCase(b, gen, seg, i)
		case seg.Guard != nil:
			following := i+1 < len(gen.Segments) || len(gen.Trailer) > 0
			body, err = guardedCase(b, gen, seg, i, following)
		default:
			body, err = linearCase(b, gen, seg, i)
		}
		if err != nil {
			return nil, err
		}
		cases = append(cases, &ast.CaseClause{List: []ast.Expr{intLit(int64(i))}, Body: body})
	}

	trailer, err := b.lowerBlock(gen.Trailer)
	if err != nil {
		return nil, err
	}
	if len(gen.Trailer) > 0 {
		done := int64(len(gen.Segments))
		body := append(trailer, setStmt(sel(genRecv, stateField), intLit(done+1)))
		cases = append(cases, &ast.CaseClause{List: []ast.Expr{intLit(done)}, Body: body})
	}

	// A guard anywhere in the machine (a segment's pre-statements, its yield, a
	// loop bound, or the trailer) reaches deoptEdge, which for a paramless handler-
	// less generator builder emits a single-value `return name_deopt0()` into the
	// three-value Next: broken Go. The static generator has no boxed-frame resume at
	// M4 (doc 06 sections 8.2 and 8.3 defer materializing the resumable frame), so
	// there is nowhere for a mid-machine guard to fall back to. Refuse rather than
	// emit wrong Go; the unit stays boxed and its guard deopts through the boxed
	// generator instead.
	if b.deoptUsed || b.nDeopt > 0 {
		return nil, fmt.Errorf("emit: generator %s carries a guard with no static deopt edge; keep it boxed", gen.Name)
	}

	stmts := []ast.Stmt{
		&ast.SwitchStmt{Tag: sel(genRecv, stateField), Body: block(cases...)},
		ret(gen.Elem.zero(), ident("true"), ident("nil")),
	}
	return &ast.FuncDecl{
		Recv: fieldList(field(&ast.StarExpr{X: ident(gen.Name)}, genRecv)),
		Name: ident("Next"),
		Type: &ast.FuncType{
			Params: &ast.FieldList{},
			Results: fieldList(
				field(gen.Elem.goType()),
				field(ident("bool")),
				field(ident("error")),
			),
		},
		Body: block(stmts...),
	}, nil
}

// linearCase builds a straight-line segment's case: run its pre-statements, flush
// any guards the yield lowered, advance the discriminant to the next state, and
// return the yielded value with done false.
func linearCase(b *Builder, gen Generator, seg Segment, i int) ([]ast.Stmt, error) {
	pre, err := b.lowerBlock(seg.Pre)
	if err != nil {
		return nil, err
	}
	val, err := lowerYield(b, gen, seg.Yield)
	if err != nil {
		return nil, err
	}
	flushed := b.flush()
	body := append(pre, flushed...)
	body = append(body,
		setStmt(sel(genRecv, stateField), intLit(int64(i+1))),
		ret(val, ident("false"), ident("nil")),
	)
	return body, nil
}

// loopCase builds a counting loop turned inside out. The case guards the loop body
// on `g.<induction> < <bound>`: while in range it runs the body, captures the yield
// into a local so the increment cannot disturb it, advances the saved counter, and
// returns without changing the discriminant so the next call re-enters the same
// state at the next counter value. When the counter reaches the bound the guard
// falls through and the machine advances to the following state.
func loopCase(b *Builder, gen Generator, seg Segment, i int) ([]ast.Stmt, error) {
	boundVal, _, err := b.lowerExpr(seg.Loop.Bound)
	if err != nil {
		return nil, err
	}
	boundFlush := b.flush()
	pre, err := b.lowerBlock(seg.Pre)
	if err != nil {
		return nil, err
	}
	val, err := lowerYield(b, gen, seg.Yield)
	if err != nil {
		return nil, err
	}
	yieldFlush := b.flush()

	loopBody := append([]ast.Stmt{}, pre...)
	loopBody = append(loopBody, yieldFlush...)
	loopBody = append(loopBody,
		define(loopYieldTmp, val),
		&ast.IncDecStmt{X: sel(genRecv, seg.Loop.Induction), Tok: token.INC},
		ret(ident(loopYieldTmp), ident("false"), ident("nil")),
	)

	cond := binary(token.LSS, sel(genRecv, seg.Loop.Induction), boundVal)
	body := append([]ast.Stmt{}, boundFlush...)
	body = append(body,
		ifStmt(cond, loopBody...),
		setStmt(sel(genRecv, stateField), intLit(int64(i+1))),
	)
	return body, nil
}

// guardedCase builds a yield sitting inside an `if`. When the guard holds the case
// runs the pre-statements, advances the discriminant past this segment, and returns
// the yield with done false. When the guard does not hold the yield is skipped: if a
// following segment exists the case falls through to it in the same call, so the
// generator produces the following value with no wasted Next; if this is the last
// segment the case advances to the done state instead, because Go forbids a
// fallthrough in the final clause and the machine has nothing left to yield.
func guardedCase(b *Builder, gen Generator, seg Segment, i int, following bool) ([]ast.Stmt, error) {
	guardVal, _, err := b.lowerExpr(seg.Guard)
	if err != nil {
		return nil, err
	}
	guardFlush := b.flush()
	pre, err := b.lowerBlock(seg.Pre)
	if err != nil {
		return nil, err
	}
	val, err := lowerYield(b, gen, seg.Yield)
	if err != nil {
		return nil, err
	}
	yieldFlush := b.flush()

	thenBody := append([]ast.Stmt{}, pre...)
	thenBody = append(thenBody, yieldFlush...)
	thenBody = append(thenBody,
		setStmt(sel(genRecv, stateField), intLit(int64(i+1))),
		ret(val, ident("false"), ident("nil")),
	)

	body := append([]ast.Stmt{}, guardFlush...)
	body = append(body, ifStmt(guardVal, thenBody...))
	if following {
		body = append(body, &ast.BranchStmt{Tok: token.FALLTHROUGH})
	} else {
		body = append(body, setStmt(sel(genRecv, stateField), intLit(int64(i+1))))
	}
	return body, nil
}

// loopYieldTmp is the local a loop segment binds the yielded value to before it
// advances the counter, so the returned value is the one from before the bump.
const loopYieldTmp = "v"

// lowerYield lowers a segment's yield expression and applies the int-to-float
// coercion an int yield into a float generator takes, refusing a yield whose
// scalar class does not fit the declared element type.
func lowerYield(b *Builder, gen Generator, y Expr) (ast.Expr, error) {
	val, vr, err := b.lowerExpr(y)
	if err != nil {
		return nil, err
	}
	if !assignable(vr, gen.Elem) {
		return nil, fmt.Errorf("emit: generator yields %s but is declared to yield %s", vr.Scalar, gen.Elem.Scalar)
	}
	if vr.Scalar == SInt && gen.Elem.Scalar == SFloat {
		val = toFloat(val, vr)
	}
	return val, nil
}

// assignable reports whether a yielded value's representation fits the generator's
// declared element type. An int coerces into a float element the way arithmetic
// does; otherwise the scalar classes must match.
func assignable(got, want Repr) bool {
	if got.Scalar == want.Scalar {
		return true
	}
	return got.Scalar == SInt && want.Scalar == SFloat
}
