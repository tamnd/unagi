package types

import (
	"sort"

	"github.com/tamnd/unagi/pkg/frontend"
)

// This file loads a typeshed stub into lattice claims and cross-checks a
// reimplemented module against it. Two rules from doc 04 section 4.3 drive it.
// First, a stub is a bundle of claims: every signature it declares is an
// unverified assertion, exactly like a user annotation, so a call typed only
// from a stub stays on the boxed tier. Second, for a stdlib module unagi
// reimplements in Go per D10, the Go implementation is the truth and the stub is
// a check on it: CI compares the registered Go signatures against the stub and
// fails on an unexplained disagreement, with an allowlist recording the
// explained ones (a typeshed bug we filed, or a 3.14 behavior typeshed has not
// caught up to).
//
// A stub is ordinary Python syntax with elided bodies, so it loads through the
// same frontend parser rather than a second one, and its parameter and return
// annotations lower through the same Lowerer user annotations use.

// Stub is a loaded stub module: the callable signatures and annotated variables
// it declares, both as claims, plus the two escape hatches that make an
// undeclared name a Dyn claim rather than an error. Getattr is set by a
// module-level __getattr__, Partial by a stub that only covers part of its
// module.
type Stub struct {
	Funcs   map[string]*Signature
	Vars    map[string]*Type
	Getattr bool
	Partial bool

	in *Interner
}

// LoadStub reads a parsed stub module into a Stub, lowering every parameter and
// return annotation through low. The file name feeds the spans on any
// degradation the lowering records. Only module-level defs and annotated
// assignments are read; nested class members are out of scope for v0.1's
// cross-check, which targets the module's public function table.
func LoadStub(mod *frontend.Module, low *Lowerer, file string) *Stub {
	s := &Stub{Funcs: map[string]*Signature{}, Vars: map[string]*Type{}, in: low.in}
	for _, stmt := range mod.Body {
		switch st := stmt.(type) {
		case *frontend.FuncDef:
			if st.Name == "__getattr__" {
				// The __getattr__ hatch declares that any name the stub omits is
				// Any, so an unknown lookup is a Dyn claim, not a miss.
				s.Getattr = true
				continue
			}
			s.Funcs[st.Name] = low.Signature(st, file)
		case *frontend.AnnAssign:
			if name, ok := st.Target.(*frontend.Name); ok {
				t, _ := low.Lower(st.Annotation, file)
				s.Vars[name.Id] = t
			}
		}
	}
	return s
}

// Lookup returns the type a name carries in the stub as a claim: a callable for
// a declared function, the annotated type for a variable, or Dyn when the stub
// omits the name but a __getattr__ or partial-stub marker makes omissions Any.
// The second result is false only for a hard miss, a name the stub neither
// declares nor covers by an escape hatch.
func (s *Stub) Lookup(name string) (*Type, bool) {
	if sig, ok := s.Funcs[name]; ok {
		return s.in.Callable(sig), true
	}
	if t, ok := s.Vars[name]; ok {
		return t, true
	}
	if s.Getattr || s.Partial {
		return s.in.Dyn(), true
	}
	return nil, false
}

// Signature lowers a def's parameters and return annotation into a lattice
// signature of claims. Parameter kinds map from the frontend's calling
// convention; *args and **kwargs move into the signature's Star and StarStar
// element types; an unannotated parameter or return is Dyn. Effect bits stay
// off, since a stub says nothing about raising or yielding.
func (l *Lowerer) Signature(fn *frontend.FuncDef, file string) *Signature {
	sig := &Signature{}
	for _, p := range fn.Params {
		t, _ := l.Lower(p.Annotation, file)
		switch p.Kind {
		case frontend.ParamStar:
			sig.Star = t
		case frontend.ParamStarStar:
			sig.StarStar = t
		default:
			sig.Params = append(sig.Params, Param{
				Name:       p.Name,
				Type:       t,
				Kind:       paramKind(p.Kind),
				HasDefault: p.Default != nil,
			})
		}
	}
	ret, _ := l.Lower(fn.Returns, file)
	sig.Return = ret
	return sig
}

// paramKind maps a frontend parameter kind to the lattice's, collapsing the
// forms the signature Params list carries. Star and StarStar are handled by the
// caller and never reach here.
func paramKind(k frontend.ParamKind) ParamKind {
	switch k {
	case frontend.ParamPosOnly:
		return ParamPosOnly
	case frontend.ParamKwOnly:
		return ParamKwOnly
	default:
		return ParamPosOrKw
	}
}

// Disagreement is one cross-check failure: a reimplemented function whose Go
// signature does not match the stub, or one the stub does not declare at all.
// Reason is a short phrase for the CI message.
type Disagreement struct {
	Name       string
	Registered *Signature
	Stub       *Signature
	Reason     string
}

// CrossCheck compares a module's registered Go signatures against its stub and
// returns the unexplained disagreements, in name order for a stable CI report.
// A name on the allowlist is skipped, whatever it does. A registered function
// the stub declares differently is a mismatch; one the stub omits is a
// disagreement too, unless the stub's escape hatches make omissions Any. A stub
// function the module did not reimplement is not a disagreement: that code just
// stays boxed.
func CrossCheck(registered map[string]*Signature, stub *Stub, allow map[string]bool) []Disagreement {
	names := make([]string, 0, len(registered))
	for name := range registered {
		names = append(names, name)
	}
	sort.Strings(names)

	var out []Disagreement
	for _, name := range names {
		if allow[name] {
			continue
		}
		reg := registered[name]
		want, ok := stub.Funcs[name]
		if !ok {
			if stub.Getattr || stub.Partial {
				continue
			}
			out = append(out, Disagreement{Name: name, Registered: reg,
				Reason: "declared in Go but absent from the stub"})
			continue
		}
		if !sigShapeEqual(reg, want) {
			out = append(out, Disagreement{Name: name, Registered: reg, Stub: want,
				Reason: "signature shape disagrees with the stub"})
		}
	}
	return out
}

// sigShapeEqual reports whether two signatures agree on the shape a caller sees,
// ignoring the effect bits a stub never carries. It compares parameter counts,
// each parameter's kind, default flag, and type, and the keyword name where a
// caller could pass by keyword, plus the *args and **kwargs element types and
// the return type. Type equality is pointer equality, so both signatures must
// come from the same interner.
func sigShapeEqual(a, b *Signature) bool {
	if len(a.Params) != len(b.Params) || a.Return != b.Return ||
		a.Star != b.Star || a.StarStar != b.StarStar {
		return false
	}
	for i := range a.Params {
		pa, pb := a.Params[i], b.Params[i]
		if pa.Kind != pb.Kind || pa.HasDefault != pb.HasDefault || pa.Type != pb.Type {
			return false
		}
		// A keyword-passable parameter's name is part of the public contract.
		if pa.Kind != ParamPosOnly && pa.Name != pb.Name {
			return false
		}
	}
	return true
}
