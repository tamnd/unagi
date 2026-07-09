package emit

import (
	"fmt"
	"go/ast"
	"go/token"
)

// This file lowers the scalar statement model. Every statement flushes the guards
// its expressions produced ahead of itself, so an overflow check or a zero-divisor
// check always sits at a statement boundary, never inside the value it guards.
// That is the placement doc 06 section 8.2 requires for a resume point, and it is
// what lets a deopt hand-off replay from a clean statement boundary.

// Stmt is a node in the scalar statement model.
type Stmt interface{ isStmt() }

// genDone and genErr are the local names a generator drive loop binds the done
// flag and the D14 error to on each Next, alongside the element it yields.
const (
	genDone = "done"
	genErr  = "err"
)

// enumRawIndex is the scratch name an enumerate loop binds Go's machine-int range
// index to before converting it into the int64 counter the body reads.
const enumRawIndex = "gi"

// Define binds a fresh local to a value, `name := value`.
type Define struct {
	Name  string
	Value Expr
}

// AugAssign is an augmented arithmetic assignment, `name += value` and its `-=`
// and `*=` siblings, with Op naming the operator (OpAdd, OpSub, or OpMul). On a
// float target it lowers to Go's matching compound assignment directly, since
// float arithmetic is total; on an int target it lowers through the same
// overflow-guarded operation a written-out `name = name OP value` would, so
// accumulation cannot silently wrap. Op's zero value is OpAdd, so a plain `+=`
// need not set it.
type AugAssign struct {
	Name  string
	Op    Op
	Repr  Repr
	Value Expr
}

// Assign rebinds an already-declared local to a new value, `name = value`. It is
// the plain assignment a Python rebinding lowers to once the name exists: the first
// binding of a name is a Define (`:=`) and every later binding is an Assign (`=`),
// since Go declares a name once and reassigns it thereafter. Like every statement
// it flushes its value's guards ahead of itself.
type Assign struct {
	Name  string
	Value Expr
}

// VarDecl declares a local with its zero value and definite Go type, `var name T`.
// It hoists a name that both arms of an if/else bind so the name is visible and
// typed after the join. Both arms assign it on every path, so the zero the
// declaration gives is never observed, and no untyped Go zero leaks past the branch.
type VarDecl struct {
	Name string
	Repr Repr
}

// Bind is a parallel binding of several names to several values in one statement,
// `a, b := x, y` when Define is true and `a, b = x, y` when it is false. It lowers a
// Python tuple unpack: Go evaluates the whole right side before assigning any
// target, the same order Python's unpack uses, so a swap `a, b = b, a` binds
// correctly with no temp. Each value's guards flush ahead of the statement, so every
// bound value is proven before it lands.
type Bind struct {
	Names  []string
	Values []Expr
	Define bool
}

// Return returns a value on the success path, lowered to `return value, nil`.
type Return struct{ Value Expr }

// ForRange iterates a slice, `for _, bind := range over { body }`. Over must lower
// to a list representation; bind takes the element representation for the body.
//
// Index, when set, is the `for i, x in enumerate(xs)` form: the loop also binds a
// counter named Index. Go's range index is a machine int, but the int
// representation the body reads is int64, so the counter is converted once at the
// top of each turn into the named int64 binding. A slice can never hold more than
// an int's worth of elements, so the widening conversion is always exact.
type ForRange struct {
	Bind  string
	Index string
	Over  Expr
	Body  []Stmt
}

// ForGen drives a static generator to exhaustion, the shape `for bind in gen(): body`
// takes when gen is a proven static generator. Gen lowers to the generator handle
// (a pointer to the state-machine struct doc 08 emits); each turn calls its Next,
// propagates a D14 error, breaks on the done flag, and otherwise binds the yielded
// element to Bind at the element representation. Bind takes the unboxed element
// Repr, never objects.Object, so a scalar generator consumed by a static for stays
// static across the consume boundary with nothing boxed. Bind is re-bound every
// turn, so the body reads the current element.
type ForGen struct {
	Bind string
	Elem Repr
	Gen  Expr
	Body []Stmt
}

// ForCount is the counting loop `for i := start; i < stop; i++ { body }`, the shape
// `for i in range(start, stop)` lowers to. The induction variable is int64, so the
// body reads it as an int, and the loop runs while it is below the stop bound. The stop
// bound is a plain value the bridge proves cheap to re-evaluate (a name or a literal),
// so re-testing it each iteration is sound; a side-effecting bound is hoisted to a temp
// ahead of the loop (doc 06 line 50) so the header still tests a plain value.
//
// Down selects the direction. A default (ascending) loop counts up while `i < stop` with
// an `i++` step, the shape `range(start, stop)` and an explicit `+1` step take. A Down
// loop counts down while `i > stop` with an `i--` step, the shape `range(start, stop, -1)`
// takes, so a descending range terminates on the correct side of the bound. Only a step of
// magnitude one lands here: a larger step could carry the induction past int64's range
// before the bound test fires, an overflow the bridge keeps boxed until the loop-back-edge
// resume point lands (doc 06 line 46).
type ForCount struct {
	Var   string
	Start Expr
	Stop  Expr
	Down  bool
	Body  []Stmt
	// Resume, when set, makes an overflow guard inside this loop body deopt by
	// re-entering the boxed twin at the current iteration rather than re-running
	// the whole unit from the top. It names the resume hand-off and the loop
	// carried accumulators handed to it live at the guard, so the twin resumes
	// with the same state the native loop held and skips the iterations already
	// run. The build sets it only for the canonical single-accumulator counting
	// loop it proves safe to resume; every other loop leaves it nil and keeps the
	// from-top edge, which is always correct.
	Resume *ResumeInfo
}

// ResumeInfo is the mid-loop resume plan for one counting loop: the hand-off the
// guard tail-calls and the carried accumulator names it passes, in the twin's
// parameter order after the loop counter. The counter and the entry-parameter
// snapshots are added by the emitter, so this carries only the names that live in
// the loop body.
type ResumeInfo struct {
	Handler string
	Carried []string
}

// While is a `for cond { body }` loop. Cond lowers through the shared truthiness
// rule, the same one the if uses, so one scalar has one notion of falsy in a loop
// test as everywhere else. Go's `for` with a single condition is Python's `while`:
// the condition is re-tested at the top of every iteration, so a body that rebinds a
// name the condition reads drives the loop to termination. This node carries a
// guard-free condition; a condition that would flush a guard needs the loop-back-edge
// resume point (doc 06 section 8.2), which the bridge keeps boxed until that lands.
type While struct {
	Cond Expr
	Body []Stmt
}

// Break and Continue are Go's `break` and `continue`. They are only ever built
// inside a loop body, so they always land inside the `for` the While node emits.
type Break struct{}

// Continue jumps to the next iteration of the enclosing loop.
type Continue struct{}

// If is an `if cond { then } else { else }` chain. Cond lowers through the shared
// truthiness rule, so a scalar condition becomes the Go test its type calls falsy
// (an int against zero, a str against ""), and a bool condition stands on its own.
// Else may be empty; when it is exactly one nested If the printer folds it to an
// `else if` so an elif chain reads the way it was written.
type If struct {
	Cond Expr
	Then []Stmt
	Else []Stmt
}

// Discard is a bare expression statement whose value is thrown away, `f(x)` on a
// line by itself. Only an expression that can raise or has an effect reaches here:
// a pure literal or bare name is a no-op the bridge drops before emit. A discarded
// call binds its result to `_` and still checks the D14 error, so an exception the
// call raises propagates even though its value is unused; any other discarded
// expression flushes its guards and binds its value to `_`.
type Discard struct {
	Value Expr
}

func (Define) isStmt()    {}
func (Assign) isStmt()    {}
func (VarDecl) isStmt()   {}
func (Bind) isStmt()      {}
func (AugAssign) isStmt() {}
func (Return) isStmt()    {}
func (ForRange) isStmt()  {}
func (ForGen) isStmt()    {}
func (ForCount) isStmt()  {}
func (While) isStmt()     {}
func (Break) isStmt()     {}
func (Continue) isStmt()  {}
func (If) isStmt()        {}
func (Discard) isStmt()   {}

// lowerBlock lowers a run of statements, concatenating each statement's flushed
// guards and the statement itself in order.
func (b *Builder) lowerBlock(stmts []Stmt) ([]ast.Stmt, error) {
	var out []ast.Stmt
	for _, s := range stmts {
		ss, err := b.lowerStmt(s)
		if err != nil {
			return nil, err
		}
		out = append(out, ss...)
	}
	return out, nil
}

// lowerStmt lowers one statement to the guard statements it needs followed by the
// statement proper.
func (b *Builder) lowerStmt(s Stmt) ([]ast.Stmt, error) {
	switch n := s.(type) {
	case Define:
		x, xr, err := b.lowerExpr(n.Value)
		if err != nil {
			return nil, err
		}
		// A short declaration infers the binding's Go type from the value, and an
		// untyped integer constant infers Go int, not the int64 the int
		// representation promises. That binding then fails to compile the moment it
		// reaches rt.AddInt64, so an int literal on the right is pinned to int64 the
		// way floatLit already pins a whole float literal. A value that is already
		// typed (a Var, a helper temp) needs no cast.
		if xr.Scalar == SInt {
			if lit, ok := x.(*ast.BasicLit); ok && lit.Kind == token.INT {
				x = callExpr(ident("int64"), x)
			}
		}
		return append(b.flush(), define(n.Name, x)), nil

	case Assign:
		// The name is already declared, so this is a plain `name = value`, not a
		// second `:=`. The value's guards flush ahead of the assignment the same way a
		// Define's do, so the rebound value is already proven when it lands.
		x, _, err := b.lowerExpr(n.Value)
		if err != nil {
			return nil, err
		}
		return append(b.flush(), setStmt(ident(n.Name), x)), nil

	case VarDecl:
		// A bare declaration carries no value and so no guards; it just names the local
		// and its type ahead of the branch that assigns it.
		decl := &ast.DeclStmt{Decl: &ast.GenDecl{Tok: token.VAR, Specs: []ast.Spec{
			&ast.ValueSpec{Names: []*ast.Ident{ident(n.Name)}, Type: n.Repr.goType()},
		}}}
		return append(b.flush(), decl), nil

	case Bind:
		// A parallel binding lowers each value first, so every value's guards land in
		// the pending list and flush ahead of the one assignment; Go then evaluates the
		// whole right side before binding any target, matching Python's unpack order.
		if len(n.Names) != len(n.Values) {
			return nil, fmt.Errorf("emit: a parallel binding has %d names for %d values", len(n.Names), len(n.Values))
		}
		rhs := make([]ast.Expr, len(n.Values))
		for i, v := range n.Values {
			x, xr, err := b.lowerExpr(v)
			if err != nil {
				return nil, err
			}
			// The Define form declares fresh names, so an untyped int literal must be
			// pinned to int64 the same way a single Define pins it; the Assign form binds
			// names whose Go type is already fixed, so it needs no cast.
			if n.Define && xr.Scalar == SInt {
				if lit, ok := x.(*ast.BasicLit); ok && lit.Kind == token.INT {
					x = callExpr(ident("int64"), x)
				}
			}
			rhs[i] = x
		}
		lhs := make([]ast.Expr, len(n.Names))
		for i, name := range n.Names {
			lhs[i] = ident(name)
		}
		tok := token.ASSIGN
		if n.Define {
			tok = token.DEFINE
		}
		return append(b.flush(), &ast.AssignStmt{Lhs: lhs, Tok: tok, Rhs: rhs}), nil

	case AugAssign:
		if n.Repr.Scalar == SFloat {
			x, xr, err := b.lowerExpr(n.Value)
			if err != nil {
				return nil, err
			}
			if xr.Scalar != SFloat {
				x = toFloat(x, xr)
			}
			return append(b.flush(), augAssign(n.Name, n.Op, x)), nil
		}
		// An int target accumulates through the guarded op: name = name OP value,
		// with the overflow check the op emits flushed ahead of the assignment.
		x, _, err := b.lowerExpr(Bin{Op: n.Op, L: Var{Name: n.Name, Repr: n.Repr}, R: n.Value})
		if err != nil {
			return nil, err
		}
		return append(b.flush(), setStmt(ident(n.Name), x)), nil

	case Discard:
		// A discarded call binds its value to `_` so nothing is left unused, and still
		// checks the error so a raise propagates. This is the clean form; routing it
		// through lowerCall would bind a value temp only to assign it away.
		if c, ok := n.Value.(Call); ok {
			args := make([]ast.Expr, len(c.Args))
			for i, a := range c.Args {
				x, _, err := b.lowerExpr(a)
				if err != nil {
					return nil, err
				}
				args[i] = x
			}
			exc := b.errName()
			call := &ast.AssignStmt{
				Lhs: []ast.Expr{ident("_"), ident(exc)},
				Tok: token.DEFINE,
				Rhs: []ast.Expr{callExpr(ident(c.Name), args...)},
			}
			check := ifStmt(binary(token.NEQ, ident(exc), ident("nil")), ret(b.ret.zero(), ident(exc)))
			return append(b.flush(), call, check), nil
		}
		// Any other discarded expression (an arithmetic value with its own guard, a
		// division with its zero check) evaluates for its guards and binds to `_`.
		x, _, err := b.lowerExpr(n.Value)
		if err != nil {
			return nil, err
		}
		return append(b.flush(), setStmt(ident("_"), x)), nil

	case Return:
		x, _, err := b.lowerExpr(n.Value)
		if err != nil {
			return nil, err
		}
		return append(b.flush(), ret(x, ident("nil"))), nil

	case ForRange:
		over, or, err := b.lowerExpr(n.Over)
		if err != nil {
			return nil, err
		}
		if or.Elem == nil {
			return nil, fmt.Errorf("emit: range needs a list operand, got %s", or.Go)
		}
		body, err := b.lowerBlock(n.Body)
		if err != nil {
			return nil, err
		}
		// A plain for-x drops the index with the blank; an enumerate binds Go's int
		// index under a scratch name and converts it once into the int64 counter the
		// body reads, so the counter carries the int representation like any other int.
		key := ident("_")
		if n.Index != "" {
			key = ident(enumRawIndex)
			idx := define(n.Index, callExpr(ident("int64"), ident(enumRawIndex)))
			body = append([]ast.Stmt{idx}, body...)
		}
		loop := &ast.RangeStmt{
			Key:   key,
			Value: ident(n.Bind),
			Tok:   token.DEFINE,
			X:     over,
			Body:  block(body...),
		}
		return append(b.flush(), loop), nil

	case ForGen:
		gen, _, err := b.lowerExpr(n.Gen)
		if err != nil {
			return nil, err
		}
		// The generator handle's guards, if any, flush ahead of the loop, so the
		// header calls Next on a proven value; the body's own guards stay inside the
		// loop the way ForRange keeps them.
		pre := b.flush()
		body, err := b.lowerBlock(n.Body)
		if err != nil {
			return nil, err
		}
		// Each turn binds the element, the done flag, and the D14 error from one Next.
		// The element takes the unboxed Elem type, so the consume boundary never boxes;
		// an error returns the function's zero and the error the way every other node
		// propagates D14; the done flag ends the loop before the element is used.
		next := &ast.AssignStmt{
			Lhs: []ast.Expr{ident(n.Bind), ident(genDone), ident(genErr)},
			Tok: token.DEFINE,
			Rhs: []ast.Expr{callExpr(&ast.SelectorExpr{X: gen, Sel: ident("Next")})},
		}
		errCheck := ifStmt(binary(token.NEQ, ident(genErr), ident("nil")), ret(b.ret.zero(), ident(genErr)))
		doneCheck := ifStmt(ident(genDone), &ast.BranchStmt{Tok: token.BREAK})
		loopBody := append([]ast.Stmt{next, errCheck, doneCheck}, body...)
		loop := &ast.ForStmt{Body: block(loopBody...)}
		return append(pre, loop), nil

	case ForCount:
		start, sr, err := b.lowerExpr(n.Start)
		if err != nil {
			return nil, err
		}
		stop, _, err := b.lowerExpr(n.Stop)
		if err != nil {
			return nil, err
		}
		// The induction variable is int64, so an untyped int-literal start is pinned to
		// int64 the same way a Define pins one, or the `i := 0` would infer Go int and the
		// `i < stop` comparison against an int64 bound would not compile.
		if sr.Scalar == SInt {
			if lit, ok := start.(*ast.BasicLit); ok && lit.Kind == token.INT {
				start = callExpr(ident("int64"), start)
			}
		}
		// Any guards the bound expressions carry flush ahead of the loop, so the loop
		// header itself is guard-free; the bridge only admits a guard-free bound, so this
		// pending list is empty today, but flushing here keeps a bound guard at a clean
		// statement boundary if one ever reaches this node.
		pre := b.flush()
		// A resume-enabled loop hands a body guard the current counter, the live
		// carried accumulators, and the entry-parameter snapshots, so the twin
		// re-enters at this iteration instead of the top. The frame is active only
		// while the body lowers, so a guard outside this loop is unaffected.
		if n.Resume != nil {
			args := make([]ast.Expr, 0, 1+len(n.Resume.Carried)+len(b.params))
			args = append(args, ident(n.Var))
			for _, c := range n.Resume.Carried {
				args = append(args, ident(c))
			}
			for i := range b.params {
				args = append(args, ident(deoptParam(i)))
			}
			b.resume = append(b.resume, resumeFrame{handler: n.Resume.Handler, args: args})
		}
		body, err := b.lowerBlock(n.Body)
		if n.Resume != nil {
			b.resume = b.resume[:len(b.resume)-1]
		}
		if err != nil {
			return nil, err
		}
		// The direction sets the bound test and the step together: an ascending loop runs
		// while `i < stop` and steps `i++`, a descending loop runs while `i > stop` and
		// steps `i--`, so a `range(a, b, -1)` counts down and stops on the correct side of
		// the bound.
		cmp, step := token.LSS, token.INC
		if n.Down {
			cmp, step = token.GTR, token.DEC
		}
		loop := &ast.ForStmt{
			Init: define(n.Var, start),
			Cond: binary(cmp, ident(n.Var), stop),
			Post: &ast.IncDecStmt{X: ident(n.Var), Tok: step},
			Body: block(body...),
		}
		return append(pre, loop), nil

	case While:
		// The condition is guard-free (the bridge keeps a guarded loop condition boxed),
		// so lowering it fills no pending list and the loop test stands on its own. The
		// body's own guards, if any, stay inside the loop block, which is where a
		// loop-back-edge resume point would sit.
		cond, cr, err := b.lowerExpr(n.Cond)
		if err != nil {
			return nil, err
		}
		test, err := truthyExpr(cond, cr)
		if err != nil {
			return nil, err
		}
		guards := b.flush()
		body, err := b.lowerBlock(n.Body)
		if err != nil {
			return nil, err
		}
		loop := &ast.ForStmt{Cond: test, Body: block(body...)}
		return append(guards, loop), nil

	case Break:
		return append(b.flush(), &ast.BranchStmt{Tok: token.BREAK}), nil

	case Continue:
		return append(b.flush(), &ast.BranchStmt{Tok: token.CONTINUE}), nil

	case If:
		// The condition's guards flush ahead of the whole if, never into the
		// condition expression: doc 06 section 8.2 wants a deopt to resume at the
		// clean statement boundary before the branch, not from inside a half-tested
		// condition. Lowering the condition first fills the pending list; flushing
		// after captures exactly those guards and leaves the list empty for the arms,
		// whose own guards stay inside their blocks.
		cond, cr, err := b.lowerExpr(n.Cond)
		if err != nil {
			return nil, err
		}
		test, err := truthyExpr(cond, cr)
		if err != nil {
			return nil, err
		}
		guards := b.flush()
		then, err := b.lowerBlock(n.Then)
		if err != nil {
			return nil, err
		}
		ifst := &ast.IfStmt{Cond: test, Body: block(then...)}
		if len(n.Else) > 0 {
			els, err := b.lowerBlock(n.Else)
			if err != nil {
				return nil, err
			}
			// A guard-free nested if lowers to a single statement, which folds into an
			// `else if`; anything else (a guarded nested condition flushes statements
			// ahead of its if, or a plain else body) stays a braced `else` block.
			if inner, ok := soleIf(els); ok {
				ifst.Else = inner
			} else {
				ifst.Else = block(els...)
			}
		}
		return append(guards, ifst), nil
	}
	return nil, fmt.Errorf("emit: unknown statement node %T", s)
}

// truthyExpr lowers a value to the Go boolean test its Python truthiness defines,
// the single rule doc 05 shares across `if`, `while`, and the connectives so one
// scalar has one notion of falsy everywhere. A bool is already the test; an int is
// falsy at zero, a float at zero, a string when empty, and a list when it has no
// elements. A representation with no truthiness form is refused rather than guessed.
func truthyExpr(x ast.Expr, r Repr) (ast.Expr, error) {
	switch r.Scalar {
	case SBool:
		return x, nil
	case SInt:
		return binary(token.NEQ, x, intLit(0)), nil
	case SFloat:
		return binary(token.NEQ, x, floatLit(0)), nil
	case SStr:
		return binary(token.NEQ, x, strLit("")), nil
	case NotScalar:
		if r.Elem != nil {
			return binary(token.NEQ, callExpr(ident("len"), x), intLit(0)), nil
		}
	}
	return nil, fmt.Errorf("emit: no truthiness lowering for %s", r.Scalar)
}

// soleIf reports whether a lowered block is exactly one if statement, the shape an
// elif produces when its condition needs no guard flushed ahead of it. Only then
// can the printer fold the block into an `else if`; a block carrying guard
// statements before its if keeps its braces so the guards stay ahead of the test.
func soleIf(stmts []ast.Stmt) (*ast.IfStmt, bool) {
	if len(stmts) != 1 {
		return nil, false
	}
	inner, ok := stmts[0].(*ast.IfStmt)
	return inner, ok
}
