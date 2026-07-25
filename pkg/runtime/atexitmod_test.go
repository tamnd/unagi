package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// TestAtexitRegisterRunLIFO checks register/unregister/_ncallbacks and that
// RunAtexit fires the callbacks last-in-first-out and clears the list.
func TestAtexitRegisterRunLIFO(t *testing.T) {
	atexitFuncs = nil

	var order []string
	mk := func(tag string) objects.Object {
		return objects.NewFunc(tag, 0, func(args []objects.Object) (objects.Object, error) {
			order = append(order, tag)
			return objects.None, nil
		})
	}
	a, b, c := mk("a"), mk("b"), mk("c")

	if _, err := atexitRegister([]objects.Object{a}, nil, nil); err != nil {
		t.Fatalf("register a: %v", err)
	}
	if _, err := atexitRegister([]objects.Object{b}, nil, nil); err != nil {
		t.Fatalf("register b: %v", err)
	}
	// b registered twice, then removed in full by unregister.
	if _, err := atexitRegister([]objects.Object{b}, nil, nil); err != nil {
		t.Fatalf("register b again: %v", err)
	}
	if _, err := atexitRegister([]objects.Object{c}, nil, nil); err != nil {
		t.Fatalf("register c: %v", err)
	}
	if n, _ := atexitNcallbacks(nil); objects.Repr(n) != "4" {
		t.Errorf("_ncallbacks = %s, want 4", objects.Repr(n))
	}
	if _, err := atexitUnregister([]objects.Object{b}); err != nil {
		t.Fatalf("unregister b: %v", err)
	}
	if n, _ := atexitNcallbacks(nil); objects.Repr(n) != "2" {
		t.Errorf("_ncallbacks after unregister = %s, want 2", objects.Repr(n))
	}

	RunAtexit()
	// LIFO over what remains, a and c: c ran first, then a.
	if len(order) != 2 || order[0] != "c" || order[1] != "a" {
		t.Errorf("run order = %v, want [c a]", order)
	}
	if n, _ := atexitNcallbacks(nil); objects.Repr(n) != "0" {
		t.Errorf("_ncallbacks after run = %s, want 0", objects.Repr(n))
	}
}
