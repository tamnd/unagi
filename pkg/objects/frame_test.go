package objects

import "testing"

// newTestFrame builds a frame the way runtime.PushFrame does, letting the stack
// wire f_back, so a test reads the same shape compiled code produces.
func newTestFrame(name string, optimized bool) *frameObject {
	return NewFrame(nil, nil, "t.py", name, name, 1, optimized)
}

// TestFrameStackPushLinksBack proves a push links f_back to the running top, so
// the shadow stack mirrors the caller chain sys._getframe walks.
func TestFrameStackPushLinksBack(t *testing.T) {
	th := NewThread("t", false)
	if len(th.frames) != 0 {
		t.Fatalf("fresh thread already has a frame")
	}
	outer := newTestFrame("outer", true)
	inner := newTestFrame("inner", true)
	th.PushFrame(outer)
	th.PushFrame(inner)
	if inner.back != outer {
		t.Fatalf("inner.f_back is not the caller frame")
	}
	if outer.back != nil {
		t.Fatalf("the bottom frame has a caller")
	}
}

// TestFrameAtDepthWalksAndClamps checks depth 0 is the running frame, a deeper
// depth walks toward the bottom, a negative depth clamps to 0, and a depth past
// the bottom is the ValueError CPython raises.
func TestFrameAtDepthWalksAndClamps(t *testing.T) {
	th := NewThread("t", false)
	bottom := newTestFrame("bottom", true)
	top := newTestFrame("top", true)
	th.PushFrame(bottom)
	th.PushFrame(top)

	got, err := th.FrameAtDepth(0)
	if err != nil || got != top {
		t.Fatalf("depth 0 is not the running frame: got=%v err=%v", got, err)
	}
	got, err = th.FrameAtDepth(1)
	if err != nil || got != bottom {
		t.Fatalf("depth 1 is not the caller: got=%v err=%v", got, err)
	}
	got, err = th.FrameAtDepth(-5)
	if err != nil || got != top {
		t.Fatalf("a negative depth does not clamp to the running frame: got=%v err=%v", got, err)
	}
	if _, err := th.FrameAtDepth(2); err == nil {
		t.Fatalf("a depth past the bottom did not raise")
	}
}

// TestFrameStackPopUnwinds proves a pop drops the running frame and never
// underflows, so a stray unwind cannot corrupt the stack.
func TestFrameStackPopUnwinds(t *testing.T) {
	th := NewThread("t", false)
	th.PushFrame(newTestFrame("a", true))
	th.PushFrame(newTestFrame("b", true))
	th.PopFrame()
	if got, _ := th.FrameAtDepth(0); got.(*frameObject).code.name != "a" {
		t.Fatalf("pop did not return to the caller frame")
	}
	th.PopFrame()
	th.PopFrame() // one pop too many must not panic or underflow
	if len(th.frames) != 0 {
		t.Fatalf("stack not empty after balanced pops")
	}
}

// TestFrameGlobalsIsModuleNamespace proves f_globals reads back the defining
// module's live namespace dict (with __name__ and __file__), not the module
// object, so a stdlib walk like warnings.warn's stacklevel computation that keys
// on f_globals['__name__'] works. A frame with no module reads back an empty
// dict, the documented divergence for a plain top-level script.
func TestFrameGlobalsIsModuleNamespace(t *testing.T) {
	m := NewModule("mod", "mod.py")
	f := NewFrame(nil, m, "mod.py", "probe", "probe", 1, true)
	g, err := frameLoadAttr(f, "f_globals")
	if err != nil {
		t.Fatalf("f_globals errored: %v", err)
	}
	if g.TypeName() != "dict" {
		t.Fatalf("f_globals is %s, want dict", g.TypeName())
	}
	name, err := GetItem(g, NewStr("__name__"))
	if err != nil {
		t.Fatalf("f_globals has no __name__: %v", err)
	}
	if s, _ := AsStr(name); s != "mod" {
		t.Fatalf("f_globals['__name__'] = %q, want mod", s)
	}
	if _, err := GetItem(g, NewStr("__file__")); err != nil {
		t.Fatalf("f_globals has no __file__: %v", err)
	}

	// A frame with no module reads back an empty dict rather than erroring.
	empty, err := frameLoadAttr(NewFrame(nil, nil, "s.py", "<module>", "<module>", 1, false), "f_globals")
	if err != nil {
		t.Fatalf("f_globals on a moduleless frame errored: %v", err)
	}
	if empty.TypeName() != "dict" {
		t.Fatalf("moduleless f_globals is %s, want dict", empty.TypeName())
	}
}

// TestThreadSetLineUpdatesRunningFrame proves SetLine writes the running frame's
// f_lineno so sys._getframe().f_lineno reads the live line, and that a SetLine
// with no running frame is a no-op rather than a crash.
func TestThreadSetLineUpdatesRunningFrame(t *testing.T) {
	th := NewThread("t", false)
	th.SetLine(99) // no running frame yet, must not panic
	th.PushFrame(newTestFrame("f", true))
	th.SetLine(42)
	top, _ := th.FrameAtDepth(0)
	ln, err := frameLoadAttr(top.(*frameObject), "f_lineno")
	if err != nil {
		t.Fatalf("f_lineno errored: %v", err)
	}
	if n, _ := AsInt(ln); n != 42 {
		t.Fatalf("f_lineno = %d, want 42", n)
	}
}

// TestFrameLocalsSplit proves a function frame exposes a FrameLocalsProxy while
// a module frame exposes the namespace dict, the split _collections_abc keys on.
func TestFrameLocalsSplit(t *testing.T) {
	fn, err := frameLoadAttr(newTestFrame("f", true), "f_locals")
	if err != nil {
		t.Fatalf("f_locals on a function frame errored: %v", err)
	}
	if _, ok := fn.(*framelocalsproxyObject); !ok {
		t.Fatalf("a function frame f_locals is %T, want FrameLocalsProxy", fn)
	}
	mod, err := frameLoadAttr(newTestFrame("<module>", false), "f_locals")
	if err != nil {
		t.Fatalf("f_locals on a module frame errored: %v", err)
	}
	if mod.TypeName() != "dict" {
		t.Fatalf("a module frame f_locals is %s, want dict", mod.TypeName())
	}
}
