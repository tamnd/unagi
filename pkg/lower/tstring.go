package lower

import (
	"go/ast"
	"strings"

	"github.com/tamnd/unagi/pkg/frontend"
)

// This file lowers t-strings (PEP 750). Unlike an f-string, which formats and
// joins into one str, a t-string builds a string.templatelib.Template: a tuple
// of static string parts interleaved with Interpolation objects, each holding
// the evaluated value, the verbatim expression source, the conversion, and the
// evaluated format spec. The static parts always number one more than the
// interpolations, with empty strings filling the gaps between adjacent fields
// and at the ends, so an empty t"" is a Template whose strings are ('',).

func (f *fnCtx) tstr(e *frontend.TStr) (ast.Expr, error) {
	var strExprs, interpExprs []ast.Expr
	var cur strings.Builder
	flush := func() {
		strExprs = append(strExprs, callExpr(f.e.obj("NewStr"), strLit(cur.String())))
		cur.Reset()
	}
	for _, p := range e.Parts {
		switch p := p.(type) {
		case *frontend.FText:
			cur.WriteString(p.Text)
		case *frontend.FInterp:
			// A field closes the current static run, then contributes one
			// Interpolation; the next run starts empty.
			flush()
			in, err := f.tInterp(p)
			if err != nil {
				return nil, err
			}
			interpExprs = append(interpExprs, in)
		}
	}
	flush()
	return callExpr(f.e.obj("NewTemplate"), f.objSlice(strExprs), f.objSlice(interpExprs)), nil
}

// tInterp lowers one t-string field to an Interpolation. The value is the raw
// evaluated expression, with no conversion applied: PEP 750 leaves conversion
// and formatting to the consumer, so the Interpolation only records them. The
// format spec is evaluated the way an f-string spec is, so a nested field like
// t"{x:{w}}" resolves w into the spec string.
func (f *fnCtx) tInterp(p *frontend.FInterp) (ast.Expr, error) {
	v, err := f.expr(p.X)
	if err != nil {
		return nil, err
	}
	conv := p.Conv
	if p.Eq != "" && conv == 0 && !p.HasSpec {
		// The self-documenting form defaults to repr, the same rule f-strings use.
		conv = 'r'
	}
	convExpr := f.e.obj("None")
	if conv != 0 {
		convExpr = callExpr(f.e.obj("NewStr"), strLit(string(conv)))
	}
	var specExpr ast.Expr = callExpr(f.e.obj("NewStr"), strLit(""))
	if p.HasSpec {
		specExpr, err = f.fParts(p.Spec)
		if err != nil {
			return nil, err
		}
	}
	return callExpr(f.e.obj("NewInterpolation"), v, strLit(p.Expr), convExpr, specExpr), nil
}
