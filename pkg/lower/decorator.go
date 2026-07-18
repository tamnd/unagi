package lower

import (
	"go/ast"

	"github.com/tamnd/unagi/pkg/frontend"
)

// decorate lowers a decorated def or class. CPython evaluates the decorator
// expressions top to bottom, then builds the function or class object, then
// applies the decorators bottom up, each application a plain call of the
// decorator on the object. build emits the object construction and returns
// the object expression; it runs after every decorator has been evaluated so
// the def's own defaults evaluate in the probed order (decorators first).
//
// A decorator cites its own source line for both its evaluation and its
// application: an undefined decorator name and a decorator that raises when
// called both point the traceback at the at-sign line, not the def line.
func (f *fnCtx) decorate(decos []frontend.Expr, build func() (ast.Expr, error)) (ast.Expr, error) {
	saved := f.line
	type applied struct {
		tmp  string
		line int
	}
	ds := make([]applied, 0, len(decos))
	for _, d := range decos {
		if p := d.Span(); p.Line > 0 {
			f.line = p.Line
		}
		v, err := f.expr(d)
		if err != nil {
			return nil, err
		}
		t := f.tmpVar()
		f.add(define(ident(t), v))
		ds = append(ds, applied{tmp: t, line: f.line})
	}

	// The object build (defaults, the function or class construction) runs at
	// the def statement's own line, which stmt already set.
	f.line = saved
	obj, err := build()
	if err != nil {
		return nil, err
	}

	// Apply bottom up: the innermost decorator wraps the object first.
	for i := len(ds) - 1; i >= 0; i-- {
		f.line = ds[i].line
		t := f.tmpVar()
		f.fallible(t, f.e.obj("CallT"), threadArg(), ident(ds[i].tmp), f.objSlice([]ast.Expr{obj}))
		obj = ident(t)
	}
	f.line = saved
	return obj, nil
}
