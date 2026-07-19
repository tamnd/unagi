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
