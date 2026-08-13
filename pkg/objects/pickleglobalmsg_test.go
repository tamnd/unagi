package objects

import "testing"

// TestPickleQualnameHasLocals pins the distinction CPython's whichmodule draws:
// only a qualname with a <locals> path segment is a local object. A top-level
// lambda or generator expression carries an angle-bracket name that is not a
// <locals> segment, so it is not local and hits the reachability message instead.
func TestPickleQualnameHasLocals(t *testing.T) {
	local := []string{
		"outer.<locals>.inner",
		"mk.<locals>.C",
		"f.<locals>.g.<locals>.h",
		"outer.<locals>.<listcomp>",
	}
	for _, q := range local {
		if !pickleQualnameHasLocals(q) {
			t.Errorf("pickleQualnameHasLocals(%q) = false, want true", q)
		}
	}
	notLocal := []string{
		"<lambda>",
		"<genexpr>",
		"<listcomp>",
		"top_fn",
		"Outer.method",
		"Outer.Inner",
	}
	for _, q := range notLocal {
		if pickleQualnameHasLocals(q) {
			t.Errorf("pickleQualnameHasLocals(%q) = true, want false", q)
		}
	}
}

// TestPickleReachabilityError pins the two reachability messages: a name the
// module does not hold is not found, a name holding a different object is not the
// same object, both rendered with the object's repr.
func TestPickleReachabilityError(t *testing.T) {
	o := NewInt(5) // any object; only its repr matters here
	notFound := pickleReachabilityError(o, "__main__", "<lambda>", false)
	if got, want := notFound.Error(), "PicklingError: Can't pickle 5: it's not found as __main__.<lambda>"; got != want {
		t.Errorf("not-found = %q, want %q", got, want)
	}
	notSame := pickleReachabilityError(o, "mod", "Q", true)
	if got, want := notSame.Error(), "PicklingError: Can't pickle 5: it's not the same object as mod.Q"; got != want {
		t.Errorf("not-same = %q, want %q", got, want)
	}
}
