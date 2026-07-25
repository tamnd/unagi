package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// symtableConst reads an integer constant off the _symtable module.
func symtableConst(t *testing.T, name string) int64 {
	t.Helper()
	mo, err := ImportModule("_symtable")
	if err != nil {
		t.Fatalf("import _symtable: %v", err)
	}
	v, err := objects.LoadAttr(mo, name)
	if err != nil {
		t.Fatalf("_symtable.%s: %v", name, err)
	}
	n, ok := objects.AsInt(v)
	if !ok {
		t.Fatalf("_symtable.%s is not an int", name)
	}
	return n
}

// TestSymtableConstants checks the flag block symtable.py unpacks at import
// carries the fixed CPython 3.14.6 values.
func TestSymtableConstants(t *testing.T) {
	want := map[string]int64{
		"USE": 16, "DEF_GLOBAL": 1, "DEF_NONLOCAL": 8, "DEF_LOCAL": 2,
		"DEF_PARAM": 4, "DEF_TYPE_PARAM": 1024, "DEF_FREE_CLASS": 64,
		"DEF_IMPORT": 128, "DEF_BOUND": 134, "DEF_ANNOT": 256,
		"DEF_COMP_ITER": 512, "DEF_COMP_CELL": 2048,
		"SCOPE_OFF": 12, "SCOPE_MASK": 15,
		"FREE": 4, "LOCAL": 1, "GLOBAL_IMPLICIT": 3, "GLOBAL_EXPLICIT": 2, "CELL": 5,
		"TYPE_FUNCTION": 0, "TYPE_CLASS": 1, "TYPE_MODULE": 2, "TYPE_ANNOTATION": 3,
		"TYPE_TYPE_ALIAS": 4, "TYPE_TYPE_PARAMETERS": 5, "TYPE_TYPE_VARIABLE": 6,
	}
	for name, w := range want {
		if got := symtableConst(t, name); got != w {
			t.Errorf("_symtable.%s = %d, want %d", name, got, w)
		}
	}
}

// TestSymtableSymtableRaises checks that building a table, which needs a runtime
// parser the AOT build does not carry, raises rather than fabricating a table.
func TestSymtableSymtableRaises(t *testing.T) {
	mo, err := ImportModule("_symtable")
	if err != nil {
		t.Fatalf("import _symtable: %v", err)
	}
	fn, err := objects.LoadAttr(mo, "symtable")
	if err != nil {
		t.Fatalf("_symtable.symtable: %v", err)
	}
	_, err = objects.Call(fn, []objects.Object{
		objects.NewStr("x = 1"), objects.NewStr("<string>"), objects.NewStr("exec"),
	})
	if err == nil {
		t.Fatalf("_symtable.symtable should raise, returned no error")
	}
	if ex, ok := err.(*objects.Exception); !ok || ex.Kind != "NotImplementedError" {
		t.Fatalf("_symtable.symtable raised %v, want NotImplementedError", err)
	}
}
