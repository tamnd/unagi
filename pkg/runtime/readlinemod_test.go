package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// TestReadlineHistory exercises the faithful half of the readline shim: the
// history list and the completer state a program's result can depend on.
func TestReadlineHistory(t *testing.T) {
	m := objects.NewModule("readline", "<readline>")
	if err := initReadline(m); err != nil {
		t.Fatalf("initReadline: %v", err)
	}

	call := func(name string, args ...objects.Object) objects.Object {
		t.Helper()
		f, err := objects.LoadAttr(m, name)
		if err != nil {
			t.Fatalf("LoadAttr %s: %v", name, err)
		}
		r, err := objects.Call(f, args)
		if err != nil {
			t.Fatalf("call %s: %v", name, err)
		}
		return r
	}
	// The module keeps global state; start from a clean slate.
	call("clear_history")

	call("add_history", objects.NewStr("one"))
	call("add_history", objects.NewStr("two"))
	call("add_history", objects.NewStr("three"))

	if n, _ := objects.AsInt(call("get_current_history_length")); n != 3 {
		t.Fatalf("current length = %d, want 3", n)
	}
	// get_history_item is 1-based.
	if s, _ := objects.AsStr(call("get_history_item", objects.NewInt(1))); s != "one" {
		t.Fatalf("item 1 = %q, want one", s)
	}
	if s, _ := objects.AsStr(call("get_history_item", objects.NewInt(3))); s != "three" {
		t.Fatalf("item 3 = %q, want three", s)
	}
	// Out of range yields None, not an error.
	if got := call("get_history_item", objects.NewInt(9)); got != objects.None {
		t.Fatalf("item 9 = %v, want None", got)
	}

	call("replace_history_item", objects.NewInt(1), objects.NewStr("TWO"))
	if s, _ := objects.AsStr(call("get_history_item", objects.NewInt(2))); s != "TWO" {
		t.Fatalf("item 2 after replace = %q, want TWO", s)
	}

	call("remove_history_item", objects.NewInt(0))
	if n, _ := objects.AsInt(call("get_current_history_length")); n != 2 {
		t.Fatalf("length after remove = %d, want 2", n)
	}
	if s, _ := objects.AsStr(call("get_history_item", objects.NewInt(1))); s != "TWO" {
		t.Fatalf("item 1 after remove = %q, want TWO", s)
	}

	call("clear_history")
	if n, _ := objects.AsInt(call("get_current_history_length")); n != 0 {
		t.Fatalf("length after clear = %d, want 0", n)
	}
}

// TestReadlineCompleterAndDegradedNoops checks the completer round-trips and the
// interactive surface reports honest empty/no-op values.
func TestReadlineCompleterAndDegradedNoops(t *testing.T) {
	m := objects.NewModule("readline", "<readline>")
	if err := initReadline(m); err != nil {
		t.Fatalf("initReadline: %v", err)
	}
	call := func(name string, args ...objects.Object) objects.Object {
		t.Helper()
		f, err := objects.LoadAttr(m, name)
		if err != nil {
			t.Fatalf("LoadAttr %s: %v", name, err)
		}
		r, err := objects.Call(f, args)
		if err != nil {
			t.Fatalf("call %s: %v", name, err)
		}
		return r
	}

	// No completer is installed by default.
	if got := call("get_completer"); got != objects.None {
		t.Fatalf("default completer = %v, want None", got)
	}
	comp := objects.NewFunc("comp", -1, func([]objects.Object) (objects.Object, error) { return objects.None, nil })
	call("set_completer", comp)
	if got := call("get_completer"); got != comp {
		t.Fatalf("completer did not round-trip")
	}

	// Delimiters default to the GNU set and round-trip a custom value.
	if s, _ := objects.AsStr(call("get_completer_delims")); s == "" {
		t.Fatalf("default delims empty")
	}
	call("set_completer_delims", objects.NewStr(" \t"))
	if s, _ := objects.AsStr(call("get_completer_delims")); s != " \t" {
		t.Fatalf("delims = %q, want ' \\t'", s)
	}

	// Degraded interactive surface: honest empties and no-ops.
	if s, _ := objects.AsStr(call("get_line_buffer")); s != "" {
		t.Fatalf("line buffer = %q, want empty", s)
	}
	if got := call("insert_text", objects.NewStr("x")); got != objects.None {
		t.Fatalf("insert_text = %v, want None", got)
	}
	if got := call("parse_and_bind", objects.NewStr("tab: complete")); got != objects.None {
		t.Fatalf("parse_and_bind = %v, want None", got)
	}
}
