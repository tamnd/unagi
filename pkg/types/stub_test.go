package types

import (
	"testing"

	"github.com/tamnd/unagi/pkg/frontend"
)

func loadStub(t *testing.T, src string) (*Stub, *Lowerer) {
	t.Helper()
	mod, err := frontend.Parse([]byte(src), "m.pyi")
	if err != nil {
		t.Fatalf("parse stub: %v", err)
	}
	in := NewInterner()
	low := NewLowerer(in, nil)
	return LoadStub(mod, low, "m.pyi"), low
}

const sampleStub = `def add(a: int, b: int) -> int: ...
def greet(name: str, sep: str = " ") -> str: ...
count: int
def __getattr__(name: str) -> Any: ...
`

func TestStubLoad(t *testing.T) {
	s, _ := loadStub(t, sampleStub)

	add, ok := s.Lookup("add")
	if !ok || add.String() != "(int, int) -> int" {
		t.Fatalf("add signature = %v", add)
	}
	greet, _ := s.Lookup("greet")
	if greet.String() != "(str, str=?) -> str" {
		t.Fatalf("greet signature = %s", greet)
	}
	// A module-level variable annotation is a claim of its type.
	c, ok := s.Lookup("count")
	if !ok || c.String() != "int" {
		t.Fatalf("count = %v", c)
	}
	// The __getattr__ hatch makes any undeclared name a Dyn claim, not a miss.
	if !s.Getattr {
		t.Fatalf("stub should have detected __getattr__")
	}
	miss, ok := s.Lookup("whatever")
	if !ok || !miss.IsDyn() {
		t.Fatalf("undeclared name under __getattr__ should be Dyn, got %v", miss)
	}
}

func TestStubHardMiss(t *testing.T) {
	s, _ := loadStub(t, "def only(a: int) -> int: ...\n")
	if _, ok := s.Lookup("absent"); ok {
		t.Fatalf("a stub with no hatch should hard-miss an undeclared name")
	}
}

// registeredFrom parses a module of defs and builds the signature table a
// reimplemented module would register, using the given interner so its types
// share identity with the stub's.
func registeredFrom(t *testing.T, in *Interner, src string) map[string]*Signature {
	t.Helper()
	mod, err := frontend.Parse([]byte(src), "impl.py")
	if err != nil {
		t.Fatalf("parse impl: %v", err)
	}
	low := NewLowerer(in, nil)
	out := map[string]*Signature{}
	for _, stmt := range mod.Body {
		if fn, ok := stmt.(*frontend.FuncDef); ok {
			out[fn.Name] = low.Signature(fn, "impl.py")
		}
	}
	return out
}

func TestCrossCheckAgrees(t *testing.T) {
	// The Go implementation matches the stub exactly, so there is no
	// disagreement. Both tables lower through the same interner.
	mod, _ := frontend.Parse([]byte("def add(a: int, b: int) -> int: ...\n"), "m.pyi")
	in := NewInterner()
	low := NewLowerer(in, nil)
	stub := LoadStub(mod, low, "m.pyi")

	reg := registeredFrom(t, in, "def add(a: int, b: int) -> int: ...\n")
	if d := CrossCheck(reg, stub, nil); len(d) != 0 {
		t.Fatalf("matching signatures should not disagree, got %v", d)
	}
}

func TestCrossCheckDisagrees(t *testing.T) {
	mod, _ := frontend.Parse([]byte("def add(a: int, b: int) -> int: ...\n"), "m.pyi")
	in := NewInterner()
	low := NewLowerer(in, nil)
	stub := LoadStub(mod, low, "m.pyi")

	// The Go implementation returns str where the stub promises int.
	reg := registeredFrom(t, in, "def add(a: int, b: int) -> str: ...\n")
	d := CrossCheck(reg, stub, nil)
	if len(d) != 1 || d[0].Name != "add" {
		t.Fatalf("return mismatch should be one disagreement, got %v", d)
	}

	// The allowlist explains it away.
	if d := CrossCheck(reg, stub, map[string]bool{"add": true}); len(d) != 0 {
		t.Fatalf("allowlisted name should not disagree, got %v", d)
	}
}

func TestCrossCheckUndeclared(t *testing.T) {
	mod, _ := frontend.Parse([]byte("def add(a: int) -> int: ...\n"), "m.pyi")
	in := NewInterner()
	low := NewLowerer(in, nil)
	stub := LoadStub(mod, low, "m.pyi")

	// The module reimplements a function the stub does not declare, and the stub
	// has no escape hatch, so it is an unexplained disagreement.
	reg := registeredFrom(t, in, "def add(a: int) -> int: ...\ndef extra(x: int) -> int: ...\n")
	d := CrossCheck(reg, stub, nil)
	if len(d) != 1 || d[0].Name != "extra" {
		t.Fatalf("undeclared reimpl should disagree, got %v", d)
	}
}
