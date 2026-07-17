package ir

import (
	"strings"
	"testing"

	"github.com/tamnd/unagi/pkg/emit"
)

// genResolver lowers a generator def and returns a GeneratorResolver that reports
// it under its own name, the way the build's whole-module wiring will hand a
// consumer the generators it may drive. It fails the test if the def does not lower
// or does not read as a drivable static generator, since both are the precondition a
// drive-site test rests on.
func genResolver(t *testing.T, src, goName string) GeneratorResolver {
	t.Helper()
	fn := parseFunc(t, src)
	gen, err := LowerGenerator(fn)
	if err != nil {
		t.Fatalf("LowerGenerator: %v", err)
	}
	sig, ok := GeneratorSignatureOf(gen, fn, goName)
	if !ok {
		t.Fatalf("GeneratorSignatureOf refused %s", fn.Name)
	}
	return func(name string) (StaticGenerator, bool) {
		if name == fn.Name {
			return sig, true
		}
		return StaticGenerator{}, false
	}
}

// emitDrive lowers a consumer function with a generator resolver in hand and prints
// it, so a test can assert on the drive site the bridge builds.
func emitDrive(t *testing.T, src string, gens GeneratorResolver) (string, error) {
	t.Helper()
	fn := parseFunc(t, src)
	f, err := LowerFuncGen(fn, nil, nil, nil, gens)
	if err != nil {
		return "", err
	}
	out, err := emit.EmitFunc(f)
	if err != nil {
		t.Fatalf("EmitFunc: %v", err)
	}
	return out, nil
}

// TestLowerForGenDrivesStaticGenerator proves a `for x in countGen(n)` whose callee
// the resolver knows lowers to constructing the handle once ahead of the loop and
// looping on its Next: the handle is a fresh local bound to &countGen{n: n}, the loop
// binds the int element, breaks on done, and threads the D14 error, with nothing
// boxed across the consume boundary.
func TestLowerForGenDrivesStaticGenerator(t *testing.T) {
	gens := genResolver(t, "def countGen(n: int):\n    for i in range(n):\n        yield i\n", "countGen")
	src := "def total(n: int) -> int:\n    s = 0\n    for x in countGen(n):\n        s += x\n    return s\n"
	got, err := emitDrive(t, src, gens)
	if err != nil {
		t.Fatalf("LowerFuncGen: %v", err)
	}
	for _, want := range []string{
		"g := &countGen{n: n}",
		"x, done, err := g.Next()",
		"if done {",
		"rt.AddInt64(s, x)",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("drive site should contain %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "objects.") {
		t.Fatalf("the consume boundary must not box the element:\n%s", got)
	}
}

// TestLowerForGenConstructsHandleOnce proves the handle is built before the loop, not
// rebuilt each turn: the construction is a single Define ahead of the `for`, and the
// loop header reads the bound handle by name. A struct literal inside Next would
// reset the machine's state every call, so this is the invariant that keeps the
// sequence advancing.
func TestLowerForGenConstructsHandleOnce(t *testing.T) {
	gens := genResolver(t, "def countGen(n: int):\n    for i in range(n):\n        yield i\n", "countGen")
	src := "def total(n: int) -> int:\n    s = 0\n    for x in countGen(n):\n        s += x\n    return s\n"
	got, err := emitDrive(t, src, gens)
	if err != nil {
		t.Fatalf("LowerFuncGen: %v", err)
	}
	construct := strings.Index(got, "&countGen{")
	loop := strings.Index(got, "for {")
	if construct < 0 || loop < 0 {
		t.Fatalf("expected a construction and a bare for loop:\n%s", got)
	}
	if construct > loop {
		t.Fatalf("the handle must be constructed before the loop, not inside it:\n%s", got)
	}
	if strings.Count(got, "&countGen{") != 1 {
		t.Fatalf("the handle should be constructed exactly once:\n%s", got)
	}
}

// TestLowerForGenWithoutResolverStaysBoxed proves the same consumer with no
// generator resolver refuses the for over a non-range call, so the unit stays boxed.
// This is the R5-safe default: a consumer only drives a generator statically when the
// module actually proved one, otherwise it runs on the boxed goroutine tier.
func TestLowerForGenWithoutResolverStaysBoxed(t *testing.T) {
	src := "def total(n: int) -> int:\n    s = 0\n    for x in countGen(n):\n        s += x\n    return s\n"
	if _, err := emitDrive(t, src, nil); err == nil {
		t.Fatal("a for over a generator call with no resolver should refuse, not drive a machine")
	}
}

// TestGeneratorSignatureOfMarksSavedParams proves the drive-site signature reports a
// parameter as saved exactly when the machine carries a field for it: countGen reads
// its bound n across the suspension, so n is saved, while a parameter the body never
// reads across a yield is not.
func TestGeneratorSignatureOfMarksSavedParams(t *testing.T) {
	fn := parseFunc(t, "def g(a: int, b: int):\n    yield a\n")
	gen, err := LowerGenerator(fn)
	if err != nil {
		t.Fatalf("LowerGenerator: %v", err)
	}
	sig, ok := GeneratorSignatureOf(gen, fn, "g")
	if !ok {
		t.Fatalf("GeneratorSignatureOf refused g")
	}
	if len(sig.Params) != 2 {
		t.Fatalf("want two parameters, got %d", len(sig.Params))
	}
	if !sig.Params[0].Saved {
		t.Fatalf("the yielded parameter a should be saved: %+v", sig.Params[0])
	}
	if sig.Params[1].Saved {
		t.Fatalf("the unread parameter b should not be saved: %+v", sig.Params[1])
	}
}

// TestLowerForGenArityMismatchRefuses proves a call whose argument count disagrees
// with the generator signature refuses rather than constructing a handle the fields
// do not fit.
func TestLowerForGenArityMismatchRefuses(t *testing.T) {
	gens := genResolver(t, "def countGen(n: int):\n    for i in range(n):\n        yield i\n", "countGen")
	src := "def total(n: int) -> int:\n    s = 0\n    for x in countGen(n, n):\n        s += x\n    return s\n"
	if _, err := emitDrive(t, src, gens); err == nil {
		t.Fatal("a generator call with the wrong argument count should refuse")
	}
}
