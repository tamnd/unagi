package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// TestSysMonitoring checks the inert-but-honest sys.monitoring registry: the
// event flags and tool ids, the tool-id round-trip with the CPython ValueErrors,
// and the event-mask and callback bookkeeping bdb/pdb/doctest drive.
func TestSysMonitoring(t *testing.T) {
	mo, err := ImportModule("sys")
	if err != nil {
		t.Fatalf("import sys: %v", err)
	}
	mon, err := objects.LoadAttr(mo, "monitoring")
	if err != nil {
		t.Fatalf("sys.monitoring: %v", err)
	}
	attr := func(o objects.Object, name string) objects.Object {
		v, err := objects.LoadAttr(o, name)
		if err != nil {
			t.Fatalf("attr %s: %v", name, err)
		}
		return v
	}
	call := func(name string, args ...objects.Object) (objects.Object, error) {
		return objects.Call(attr(mon, name), args)
	}
	mustInt := func(o objects.Object) int64 {
		n, ok := objects.AsInt(o)
		if !ok {
			t.Fatalf("not an int: %s", objects.Str(o))
		}
		return n
	}

	// Event flags are the PEP 669 powers of two.
	events := attr(mon, "events")
	if got := mustInt(attr(events, "PY_START")); got != 1 {
		t.Errorf("events.PY_START = %d; want 1", got)
	}
	if got := mustInt(attr(events, "BRANCH")); got != 262144 {
		t.Errorf("events.BRANCH = %d; want 262144", got)
	}
	// Reserved tool ids.
	if got := mustInt(attr(mon, "OPTIMIZER_ID")); got != 5 {
		t.Errorf("OPTIMIZER_ID = %d; want 5", got)
	}
	// DISABLE and MISSING are distinct sentinels.
	if attr(mon, "DISABLE") == attr(mon, "MISSING") {
		t.Error("DISABLE and MISSING are the same object")
	}

	line := attr(events, "LINE")

	// Claim, read, and the double-claim / out-of-range errors.
	if _, err := call("use_tool_id", objects.NewInt(0), objects.NewStr("dbg")); err != nil {
		t.Fatalf("use_tool_id: %v", err)
	}
	if got := attr(mon, "get_tool"); got == nil {
		t.Fatal("get_tool missing")
	}
	tool, _ := call("get_tool", objects.NewInt(0))
	if s, _ := objects.AsStr(tool); s != "dbg" {
		t.Errorf("get_tool(0) = %q; want dbg", s)
	}
	if _, err := call("use_tool_id", objects.NewInt(0), objects.NewStr("other")); err == nil {
		t.Error("double use_tool_id did not raise")
	}
	if _, err := call("use_tool_id", objects.NewInt(6), objects.NewStr("x")); err == nil {
		t.Error("out-of-range use_tool_id did not raise")
	}

	// Events default to 0, set on the in-use tool, and error on a free one.
	ev, _ := call("get_events", objects.NewInt(0))
	if mustInt(ev) != 0 {
		t.Errorf("get_events default = %d; want 0", mustInt(ev))
	}
	if _, err := call("set_events", objects.NewInt(0), line); err != nil {
		t.Fatalf("set_events: %v", err)
	}
	ev, _ = call("get_events", objects.NewInt(0))
	if mustInt(ev) != 32 {
		t.Errorf("get_events after set = %d; want 32", mustInt(ev))
	}
	if _, err := call("set_events", objects.NewInt(3), line); err == nil {
		t.Error("set_events on a free tool did not raise")
	}

	// register_callback returns the callback it replaced.
	cb := objects.NewFunc("cb", -1, func([]objects.Object) (objects.Object, error) { return objects.None, nil })
	prev, _ := call("register_callback", objects.NewInt(0), line, cb)
	if prev != objects.None {
		t.Errorf("first register_callback prev = %s; want None", objects.Str(prev))
	}
	prev2, _ := call("register_callback", objects.NewInt(0), line, objects.None)
	if prev2 != objects.Object(cb) {
		t.Error("register_callback did not return the replaced callback")
	}

	// clear keeps the claim but drops events; free releases the id.
	if _, err := call("clear_tool_id", objects.NewInt(0)); err != nil {
		t.Fatalf("clear_tool_id: %v", err)
	}
	ev, _ = call("get_events", objects.NewInt(0))
	if mustInt(ev) != 0 {
		t.Errorf("get_events after clear = %d; want 0", mustInt(ev))
	}
	if _, err := call("free_tool_id", objects.NewInt(0)); err != nil {
		t.Fatalf("free_tool_id: %v", err)
	}
	after, _ := call("get_tool", objects.NewInt(0))
	if after != objects.None {
		t.Errorf("get_tool after free = %s; want None", objects.Str(after))
	}
}
