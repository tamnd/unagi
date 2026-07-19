package objects

import "testing"

// TestContextVarPerThread checks that a ContextVar set on one thread is
// invisible to another, the isolation the per-Thread current context gives.
func TestContextVarPerThread(t *testing.T) {
	v := NewContextVar("x", false, nil).(*contextVar)
	t1 := NewThread("t1", false)
	t2 := NewThread("t2", false)

	if _, err := contextVarMethod(t1, v, "set", []Object{NewInt(1)}); err != nil {
		t.Fatalf("set on t1: %v", err)
	}
	got, err := contextVarMethod(t1, v, "get", nil)
	if err != nil {
		t.Fatalf("get on t1: %v", err)
	}
	if Repr(got) != "1" {
		t.Errorf("t1 get = %s, want 1", Repr(got))
	}
	// t2 never set the variable, so get raises rather than seeing t1's value.
	if _, err := contextVarMethod(t2, v, "get", nil); !isKind(err, "LookupError") {
		t.Errorf("t2 get error = %v, want LookupError", err)
	}
}

// TestContextRunRestores checks Context.run swaps the current context for the
// call and restores the caller's context afterward, even when the variable was
// unset before the run.
func TestContextRunRestores(t *testing.T) {
	th := NewThread("main", false)
	v := NewContextVar("v", true, NewInt(0)).(*contextVar)
	ctx := NewEmptyContext().(*contextObject)

	body := NewFuncT("body", 0, func(bt *Thread, _ []Object) (Object, error) {
		return contextVarMethod(bt, v, "set", []Object{NewInt(9)})
	})
	if _, err := ctx.run(th, []Object{body}, nil, nil); err != nil {
		t.Fatalf("run: %v", err)
	}
	// Outside the run the variable is back to its default, the set stayed in ctx.
	got, err := contextVarMethod(th, v, "get", nil)
	if err != nil {
		t.Fatalf("get after run: %v", err)
	}
	if Repr(got) != "0" {
		t.Errorf("after run get = %s, want 0 (default)", Repr(got))
	}
	if inside, _ := ctx.lookup(v); inside == nil || Repr(inside) != "9" {
		t.Errorf("ctx did not retain the set from run")
	}
}
