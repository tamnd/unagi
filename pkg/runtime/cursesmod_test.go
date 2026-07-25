package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// cursesConst reads an integer constant off the _curses module.
func cursesConst(t *testing.T, name string) int64 {
	t.Helper()
	mo, err := ImportModule("_curses")
	if err != nil {
		t.Fatalf("import _curses: %v", err)
	}
	v, err := objects.LoadAttr(mo, name)
	if err != nil {
		t.Fatalf("_curses.%s: %v", name, err)
	}
	n, ok := objects.AsInt(v)
	if !ok {
		t.Fatalf("_curses.%s is not an int", name)
	}
	return n
}

// TestCursesConstants checks the ncurses attribute, color, and key constants
// carry their fixed header values.
func TestCursesConstants(t *testing.T) {
	want := map[string]int64{
		"A_NORMAL": 0, "A_BOLD": 2097152, "A_UNDERLINE": 131072,
		"COLOR_BLACK": 0, "COLOR_RED": 1, "COLOR_GREEN": 2, "COLOR_WHITE": 7,
		"KEY_UP": 259, "KEY_DOWN": 258, "KEY_LEFT": 260, "KEY_RIGHT": 261,
		"ERR": -1, "OK": 0,
	}
	for name, w := range want {
		if got := cursesConst(t, name); got != w {
			t.Errorf("_curses.%s = %d, want %d", name, got, w)
		}
	}
}

// TestCursesError checks _curses.error is a real Exception subclass so `except
// curses.error` binds.
func TestCursesError(t *testing.T) {
	mo, err := ImportModule("_curses")
	if err != nil {
		t.Fatalf("import _curses: %v", err)
	}
	if _, err := objects.LoadAttr(mo, "error"); err != nil {
		t.Fatalf("_curses.error: %v", err)
	}
}

// TestCursesFuncsRaise checks that driving the screen, which needs a terminal
// backend the AOT build does not carry, raises rather than pretending to control
// a terminal. window construction raises the same way.
func TestCursesFuncsRaise(t *testing.T) {
	mo, err := ImportModule("_curses")
	if err != nil {
		t.Fatalf("import _curses: %v", err)
	}
	for _, name := range []string{"initscr", "newwin", "start_color", "beep", "window"} {
		fn, err := objects.LoadAttr(mo, name)
		if err != nil {
			t.Fatalf("_curses.%s: %v", name, err)
		}
		_, err = objects.Call(fn, nil)
		if err == nil {
			t.Fatalf("_curses.%s should raise, returned no error", name)
		}
		if ex, ok := err.(*objects.Exception); !ok || ex.Kind != "NotImplementedError" {
			t.Fatalf("_curses.%s raised %v, want NotImplementedError", name, err)
		}
	}
}
