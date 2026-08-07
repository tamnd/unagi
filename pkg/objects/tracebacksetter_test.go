package objects

import (
	"strings"
	"testing"
)

// TestTracebackTbNextSetter checks that tb_next is writable: None clears it,
// another traceback splices in and reads back, a cycle is refused, and a
// non-traceback value is the expected-traceback TypeError.
func TestTracebackTbNextSetter(t *testing.T) {
	tb := &tracebackObject{lineno: 1}
	tb2 := &tracebackObject{lineno: 2}

	if err := StoreAttr(tb, "tb_next", tb2); err != nil {
		t.Fatalf("set tb_next to a traceback: %v", err)
	}
	got, err := LoadAttr(tb, "tb_next")
	if err != nil {
		t.Fatalf("read tb_next: %v", err)
	}
	if got != tb2 {
		t.Fatalf("tb_next = %v, want the spliced traceback", got)
	}

	if err := StoreAttr(tb, "tb_next", None); err != nil {
		t.Fatalf("set tb_next to None: %v", err)
	}
	if got, _ := LoadAttr(tb, "tb_next"); got != None {
		t.Fatalf("tb_next after None = %v, want None", got)
	}

	// A self loop is refused.
	err = StoreAttr(tb, "tb_next", tb)
	if err == nil || !strings.Contains(err.Error(), "traceback loop detected") {
		t.Fatalf("self loop error = %v, want the loop ValueError", err)
	}

	// A non-traceback value names its type.
	err = StoreAttr(tb, "tb_next", NewInt(5))
	if err == nil || !strings.Contains(err.Error(), "expected traceback object, got 'int'") {
		t.Fatalf("int store error = %v, want the type error", err)
	}
}

// TestTracebackReadonlyAndDelete checks that the other three attributes stay
// read-only with CPython's wording and that tb_next cannot be deleted.
func TestTracebackReadonlyAndDelete(t *testing.T) {
	tb := &tracebackObject{lineno: 1}
	cases := []struct {
		name string
		want string
	}{
		{"tb_lineno", "attribute 'tb_lineno' of 'traceback' objects is not writable"},
		{"tb_frame", "readonly attribute"},
		{"tb_lasti", "readonly attribute"},
	}
	for _, c := range cases {
		err := StoreAttr(tb, c.name, None)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Fatalf("set %s error = %v, want %q", c.name, err, c.want)
		}
	}
	err := DelAttr(tb, "tb_next")
	if err == nil || !strings.Contains(err.Error(), "can't delete tb_next attribute") {
		t.Fatalf("del tb_next error = %v, want the delete TypeError", err)
	}
}
