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
type Segment struct {
	Pre   []Stmt
	Yield Expr
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
		pre, err := b.lowerBlock(seg.Pre)
		if err != nil {
			return nil, err
		}
		val, vr, err := b.lowerExpr(seg.Yield)
		if err != nil {
			return nil, err
		}
		flushed := b.flush()
		if !assignable(vr, gen.Elem) {
			return nil, fmt.Errorf("emit: generator yields %s but is declared to yield %s", vr.Scalar, gen.Elem.Scalar)
		}
		if vr.Scalar == SInt && gen.Elem.Scalar == SFloat {
			val = toFloat(val, vr)
		}
		body := append(pre, flushed...)
		body = append(body,
			setStmt(sel(genRecv, stateField), intLit(int64(i+1))),
			ret(val, ident("false"), ident("nil")),
		)
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

// assignable reports whether a yielded value's representation fits the generator's
// declared element type. An int coerces into a float element the way arithmetic
// does; otherwise the scalar classes must match.
func assignable(got, want Repr) bool {
	if got.Scalar == want.Scalar {
		return true
	}
	return got.Scalar == SInt && want.Scalar == SFloat
}
