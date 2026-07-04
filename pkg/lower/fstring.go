package lower

import (
	"go/ast"
	"strings"

	"github.com/tamnd/unagi/pkg/frontend"
)

// This file lowers f-strings. Every part becomes a str object: literal runs
// are NewStr constants, interpolations run through their conversion and
// format spec, and runtime.JoinStrs glues the pieces back together.

func (f *fnCtx) fstr(e *frontend.FStr) (ast.Expr, error) {
	return f.fParts(e.Parts)
}

// fParts lowers a run of f-string parts to one str object. It backs both the
// whole f-string and a format spec that carries its own replacement fields,
// so f"{x:{w}}" builds its spec through the same path as the outer string.
func (f *fnCtx) fParts(parts []frontend.FPart) (ast.Expr, error) {
	var out []ast.Expr
	for _, p := range parts {
		switch p := p.(type) {
		case *frontend.FText:
			if p.Text == "" {
				continue
			}
			out = append(out, callExpr(f.e.obj("NewStr"), strLit(p.Text)))
		case *frontend.FInterp:
			interp, err := f.fInterp(p)
			if err != nil {
				return nil, err
			}
			out = append(out, interp...)
		}
	}
	if len(out) == 0 {
		return callExpr(f.e.obj("NewStr"), strLit("")), nil
	}
	if len(out) == 1 {
		return out[0], nil
	}
	return callExpr(sel("runtime", "JoinStrs"), out...), nil
}

// specText returns the constant text of a spec whose parts are all literal,
// letting the common f"{x:.2f}" case format against a Go string constant.
func specText(parts []frontend.FPart) (string, bool) {
	var b strings.Builder
	for _, p := range parts {
		t, ok := p.(*frontend.FText)
		if !ok {
			return "", false
		}
		b.WriteString(t.Text)
	}
	return b.String(), true
}

// fInterp lowers one interpolation to its str parts: the verbatim text for
// the self-documenting form, then the value through conversion and spec.
func (f *fnCtx) fInterp(p *frontend.FInterp) ([]ast.Expr, error) {
	var out []ast.Expr
	if p.Eq != "" {
		out = append(out, callExpr(f.e.obj("NewStr"), strLit(p.Eq)))
	}
	v, err := f.expr(p.X)
	if err != nil {
		return nil, err
	}
	conv := p.Conv
	if p.Eq != "" && conv == 0 && !p.HasSpec {
		// The self-documenting form shows repr unless a conversion or spec
		// says otherwise, per PEP 501's rules as CPython implements them.
		conv = 'r'
	}
	// The conversions can raise now that str and repr enforce the
	// 4300-digit int limit, so each goes through a fallible temp.
	fallibleConv := func(fn string) {
		tmp := f.tmpVar()
		f.fallible(tmp, sel("runtime", fn), v)
		v = ident(tmp)
	}
	switch conv {
	case 's':
		fallibleConv("StrOf")
	case 'r':
		fallibleConv("ReprOf")
	case 'a':
		fallibleConv("AsciiOf")
	}
	if p.HasSpec {
		// A conversion feeds the spec as a plain string, so str formatting
		// rules apply to the converted text, matching CPython. A literal spec
		// formats against a Go constant; a spec with its own replacement
		// fields builds a str first and formats through the runtime.
		if text, ok := specText(p.Spec); ok {
			tmp := f.tmpVar()
			f.fallible(tmp, f.e.obj("Format"), v, strLit(text))
			out = append(out, ident(tmp))
			return out, nil
		}
		spec, err := f.fParts(p.Spec)
		if err != nil {
			return nil, err
		}
		tmp := f.tmpVar()
		f.fallible(tmp, sel("runtime", "FormatSpec"), v, spec)
		out = append(out, ident(tmp))
		return out, nil
	}
	if conv == 0 {
		fallibleConv("StrOf")
	}
	out = append(out, v)
	return out, nil
}
