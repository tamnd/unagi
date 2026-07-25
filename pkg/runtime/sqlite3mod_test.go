package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// sqlite3Const reads an integer constant off the _sqlite3 module.
func sqlite3Const(t *testing.T, name string) int64 {
	t.Helper()
	mo, err := ImportModule("_sqlite3")
	if err != nil {
		t.Fatalf("import _sqlite3: %v", err)
	}
	v, err := objects.LoadAttr(mo, name)
	if err != nil {
		t.Fatalf("_sqlite3.%s: %v", name, err)
	}
	n, ok := objects.AsInt(v)
	if !ok {
		t.Fatalf("_sqlite3.%s is not an int", name)
	}
	return n
}

// TestSqlite3Constants checks the SQLite constant block carries the fixed C-header
// values (result codes, authorizer actions, limit and dbconfig ids), and that
// threadsafety is 0 since no engine is linked.
func TestSqlite3Constants(t *testing.T) {
	want := map[string]int64{
		"SQLITE_OK": 0, "SQLITE_ERROR": 1, "SQLITE_BUSY": 5, "SQLITE_LOCKED": 6,
		"SQLITE_ROW": 100, "SQLITE_DONE": 101, "SQLITE_CONSTRAINT": 19,
		"SQLITE_SELECT": 21, "SQLITE_INSERT": 18, "SQLITE_UPDATE": 23,
		"SQLITE_LIMIT_LENGTH": 0, "SQLITE_LIMIT_VARIABLE_NUMBER": 9,
		"SQLITE_DBCONFIG_ENABLE_FKEY": 1002,
		"PARSE_DECLTYPES":             1, "PARSE_COLNAMES": 2,
		"LEGACY_TRANSACTION_CONTROL": -1, "threadsafety": 0,
	}
	for name, w := range want {
		if got := sqlite3Const(t, name); got != w {
			t.Errorf("_sqlite3.%s = %d, want %d", name, got, w)
		}
	}
}

// TestSqlite3ExcHierarchy checks the DB-API exception classes chain as CPython
// does so `except sqlite3.OperationalError` and friends bind.
func TestSqlite3ExcHierarchy(t *testing.T) {
	mo, err := ImportModule("_sqlite3")
	if err != nil {
		t.Fatalf("import _sqlite3: %v", err)
	}
	load := func(name string) objects.Object {
		v, err := objects.LoadAttr(mo, name)
		if err != nil {
			t.Fatalf("_sqlite3.%s: %v", name, err)
		}
		return v
	}
	pairs := [][2]string{
		{"Error", "Exception"}, {"Warning", "Exception"},
		{"InterfaceError", "Error"}, {"DatabaseError", "Error"},
		{"DataError", "DatabaseError"}, {"OperationalError", "DatabaseError"},
		{"IntegrityError", "DatabaseError"}, {"InternalError", "DatabaseError"},
		{"ProgrammingError", "DatabaseError"}, {"NotSupportedError", "DatabaseError"},
	}
	exc, _ := objects.ExcClassValue("Exception")
	base := func(name string) objects.Object {
		if name == "Exception" {
			return exc
		}
		return load(name)
	}
	for _, p := range pairs {
		res, err := objects.IsSubclass(load(p[0]), base(p[1]))
		if err != nil {
			t.Fatalf("issubclass(%s, %s): %v", p[0], p[1], err)
		}
		if res != objects.True {
			t.Errorf("%s is not a subclass of %s", p[0], p[1])
		}
	}
}

// TestSqlite3ConnectRaises checks that opening a database, which needs the SQLite
// engine the AOT build does not carry, raises rather than fabricating one.
func TestSqlite3ConnectRaises(t *testing.T) {
	mo, err := ImportModule("_sqlite3")
	if err != nil {
		t.Fatalf("import _sqlite3: %v", err)
	}
	for _, name := range []string{"connect", "adapt", "complete_statement"} {
		fn, err := objects.LoadAttr(mo, name)
		if err != nil {
			t.Fatalf("_sqlite3.%s: %v", name, err)
		}
		_, err = objects.Call(fn, []objects.Object{objects.NewStr(":memory:")})
		if err == nil {
			t.Fatalf("_sqlite3.%s should raise, returned no error", name)
		}
		if ex, ok := err.(*objects.Exception); !ok || ex.Kind != "NotImplementedError" {
			t.Fatalf("_sqlite3.%s raised %v, want NotImplementedError", name, err)
		}
	}
}

// TestSqlite3RegisterNoop checks register_adapter and register_converter do not
// raise, since dbapi2.py calls them at import; they are inert because no engine
// consults their registrations.
func TestSqlite3RegisterNoop(t *testing.T) {
	mo, err := ImportModule("_sqlite3")
	if err != nil {
		t.Fatalf("import _sqlite3: %v", err)
	}
	for _, name := range []string{"register_adapter", "register_converter", "enable_callback_tracebacks"} {
		fn, err := objects.LoadAttr(mo, name)
		if err != nil {
			t.Fatalf("_sqlite3.%s: %v", name, err)
		}
		got, err := objects.Call(fn, []objects.Object{objects.NewStr("x"), objects.NewStr("y")})
		if err != nil {
			t.Fatalf("_sqlite3.%s should be a no-op, raised %v", name, err)
		}
		if got != objects.None {
			t.Errorf("_sqlite3.%s returned %v, want None", name, objects.Repr(got))
		}
	}
}
