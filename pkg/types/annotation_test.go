package types

import (
	"testing"

	"github.com/tamnd/unagi/pkg/frontend"
)

// parseAnn parses `x: <src>` and returns the retained annotation expression, the
// cheapest way to get a real frontend type expression into a lowering test.
func parseAnn(t *testing.T, src string) frontend.Expr {
	t.Helper()
	mod, err := frontend.Parse([]byte("x: "+src+"\n"), "t.py")
	if err != nil {
		t.Fatalf("parse %q: %v", src, err)
	}
	return mod.Body[0].(*frontend.AnnAssign).Annotation
}

type testScope struct {
	in      *Interner
	classes map[string][]string
	aliases map[string]frontend.Expr
}

func (s testScope) ResolveClass(name string) *Type {
	if mro, ok := s.classes[name]; ok {
		return s.in.Class(name, mro)
	}
	return nil
}

func (s testScope) ResolveAlias(name string) frontend.Expr {
	return s.aliases[name]
}

func TestAnnotationLowering(t *testing.T) {
	in := NewInterner()
	scope := testScope{
		in:      in,
		classes: map[string][]string{"Shape": {"Shape", "object"}},
	}
	low := NewLowerer(in, scope)

	cases := []struct {
		src  string
		want string
	}{
		{"int", "int"},
		{"Optional[int]", "None | int"},
		{"int | str", "int | str"},
		{"list[int]", "list[int]"},
		{"dict[str, int]", "dict[str, int]"},
		{"tuple[int, str]", "tuple[int, str]"},
		{"tuple[int, ...]", "tuple[int, ...]"},
		{"Callable[[int, str], bool]", "(int, str) -> bool"},
		{"Callable[..., int]", "(*Dyn) -> int"},
		{"Literal[1, 2, 3]", "int"},
		{`Literal["a", 1]`, "int | str"},
		{`Annotated[int, "meta"]`, "int"},
		{"Any", "Dyn"},
		{"list", "list[Dyn]"},
		{"Optional[Shape]", "None | Shape"},
		{"Union[int, str, bytes]", "int | str | bytes"},
	}
	for _, c := range cases {
		t.Run(c.src, func(t *testing.T) {
			got, ev := low.Lower(parseAnn(t, c.src), "t.py")
			if got.String() != c.want {
				t.Fatalf("lower %q = %s, want %s", c.src, got, c.want)
			}
			// Every lowered annotation is a claim, never a proof.
			if ev == nil || ev.IsProof() {
				t.Fatalf("lowered annotation %q should be a claim", c.src)
			}
		})
	}
}

func TestAnnotationDegrades(t *testing.T) {
	in := NewInterner()
	low := NewLowerer(in, nil)

	// An unknown name has nowhere to resolve, so it degrades to Dyn and the
	// reason is recorded with the span for the report.
	got, _ := low.Lower(parseAnn(t, "Nonexistent"), "t.py")
	if !got.IsDyn() {
		t.Fatalf("unknown name should be Dyn, got %s", got)
	}
	if len(low.Degradations()) != 1 {
		t.Fatalf("want one degradation, got %d", len(low.Degradations()))
	}
	if low.Degradations()[0].Span.Line == 0 {
		t.Fatalf("degradation should carry a source span")
	}
}

func TestAnnotationNilIsDyn(t *testing.T) {
	in := NewInterner()
	low := NewLowerer(in, nil)
	// An absent annotation is Dyn with no claim, since no hint is not a claim.
	got, ev := low.Lower(nil, "t.py")
	if !got.IsDyn() || ev != nil {
		t.Fatalf("nil annotation should be Dyn with no evidence")
	}
}

func TestAnnotationAliasCut(t *testing.T) {
	in := NewInterner()
	// A self-referential alias unfolds to the depth limit and then cuts to Dyn.
	self := parseAnn(t, "Rec")
	scope := testScope{in: in, aliases: map[string]frontend.Expr{"Rec": self}}
	low := NewLowerer(in, scope)

	got, _ := low.Lower(parseAnn(t, "Rec"), "t.py")
	if !got.IsDyn() {
		t.Fatalf("recursive alias should cut to Dyn, got %s", got)
	}
	hitCut := false
	for _, d := range low.Degradations() {
		if d.Reason == "recursive type alias cut at depth 3" {
			hitCut = true
		}
	}
	if !hitCut {
		t.Fatalf("recursive alias should record the depth cut")
	}
}

func TestAnnotationForwardRef(t *testing.T) {
	in := NewInterner()
	low := NewLowerer(in, nil)
	low.ParseForwardRef = func(s string) (frontend.Expr, error) { return parseAnn(t, s), nil }

	// A stringified annotation reparses and lowers to the same type as the bare
	// form.
	got, _ := low.Lower(parseAnn(t, `"int | None"`), "t.py")
	if got.String() != "None | int" {
		t.Fatalf("forward ref = %s, want None | int", got)
	}

	// With no parser wired the forward ref degrades.
	low2 := NewLowerer(in, nil)
	d, _ := low2.Lower(parseAnn(t, `"int"`), "t.py")
	if !d.IsDyn() {
		t.Fatalf("unwired forward ref should degrade to Dyn")
	}
}
