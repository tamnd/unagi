package types

import "testing"

func TestEnvBindAndLookup(t *testing.T) {
	in := NewInterner()
	x := Local("x")
	env := NewEnv(in)

	// An untracked place reads as Dyn.
	if !env.TypeOf(x).IsDyn() {
		t.Fatalf("untracked place should be Dyn")
	}
	env2 := env.Bind(x, in.Int(), nil)
	if got := env2.TypeOf(x).String(); got != "int" {
		t.Fatalf("bound place = %s, want int", got)
	}
	// Bind returns a new env; the original is untouched.
	if !env.TypeOf(x).IsDyn() {
		t.Fatalf("Bind mutated the receiver")
	}
	// Rebinding overwrites, which is how a reassignment ends a narrowing.
	if got := env2.Bind(x, in.Str(), nil).TypeOf(x).String(); got != "str" {
		t.Fatalf("rebind = %s, want str", got)
	}
}

func TestEnvForget(t *testing.T) {
	in := NewInterner()
	x := Local("x")
	env := NewEnv(in).Bind(x, in.Int(), nil)
	if !env.Forget(x).TypeOf(x).IsDyn() {
		t.Fatalf("Forget should drop back to Dyn")
	}
}

func TestEnvJoinKeepsOnlyCommonPlaces(t *testing.T) {
	in := NewInterner()
	x, y := Local("x"), Local("y")

	// x is int on one edge and str on the other; y is bound on one edge only.
	a := NewEnv(in).Bind(x, in.Int(), nil).Bind(y, in.Int(), nil)
	b := NewEnv(in).Bind(x, in.Str(), nil)

	m := a.Join(b)
	if got := m.TypeOf(x).String(); got != "int | str" {
		t.Fatalf("joined x = %s, want int | str", got)
	}
	// y was known on one edge only, so it is unknown after the merge.
	if !m.TypeOf(y).IsDyn() {
		t.Fatalf("y should be Dyn after a one-sided join")
	}
}

func TestEnvAfterCallKillsFragilePlaces(t *testing.T) {
	in := NewInterner()
	local := Local("n")
	attr := Attr("self.x")
	global := Global("g")

	env := NewEnv(in).
		Bind(local, in.Int(), nil).
		Bind(attr, in.Int(), nil).
		Bind(global, in.Int(), nil)

	after := env.AfterCall()
	// The local narrowing survives a call; the attribute and global do not.
	if got := after.TypeOf(local).String(); got != "int" {
		t.Fatalf("local should survive a call, got %s", got)
	}
	if !after.TypeOf(attr).IsDyn() {
		t.Fatalf("attribute narrowing should die at a call")
	}
	if !after.TypeOf(global).IsDyn() {
		t.Fatalf("global narrowing should die at a call")
	}
}

func TestEnvAfterCallNoFragileIsIdentity(t *testing.T) {
	in := NewInterner()
	local := Local("n")
	env := NewEnv(in).Bind(local, in.Int(), nil)
	// With nothing fragile to kill, AfterCall returns the same env, no copy.
	if env.AfterCall().TypeOf(local).String() != "int" {
		t.Fatalf("local-only env changed across a call")
	}
}

func TestEnvAfterYieldKillsGlobals(t *testing.T) {
	in := NewInterner()
	global := Global("g")
	local := Local("n")
	env := NewEnv(in).Bind(global, in.Int(), nil).Bind(local, in.Int(), nil)

	after := env.AfterYield()
	if !after.TypeOf(global).IsDyn() {
		t.Fatalf("global narrowing should die at a yield")
	}
	if after.TypeOf(local).String() != "int" {
		t.Fatalf("local should survive a yield")
	}
}

func TestEnvPlacesDeterministicOrder(t *testing.T) {
	in := NewInterner()
	env := NewEnv(in).
		Bind(Global("z"), in.Int(), nil).
		Bind(Local("b"), in.Int(), nil).
		Bind(Attr("a.x"), in.Int(), nil).
		Bind(Local("a"), in.Int(), nil)

	got := env.Places()
	want := []Place{Local("a"), Local("b"), Attr("a.x"), Global("z")}
	if len(got) != len(want) {
		t.Fatalf("Places count = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Places[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}
