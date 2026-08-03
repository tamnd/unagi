package objects

import (
	"strings"
	"testing"
)

func TestTracebackFromFramesChain(t *testing.T) {
	// e.Frames collects innermost (raise site) first; the projected traceback
	// heads at the outermost frame and tb_next walks inward, so a two-frame
	// exception reads back <module> then f, each with its own line, matching what
	// traceback.extract_tb yields.
	frames := []Frame{
		{File: "m.py", Line: 4, Func: "f"},        // innermost raise site
		{File: "m.py", Line: 9, Func: "<module>"}, // outermost caller
	}
	tb := tracebackFromFrames(frames)
	if tb == nil {
		t.Fatal("tracebackFromFrames returned nil")
	}
	type want struct {
		name   string
		lineno string
	}
	wants := []want{{"<module>", "9"}, {"f", "4"}}
	for i, w := range wants {
		if tb == nil {
			t.Fatalf("chain ended early at %d", i)
		}
		frameObj, err := tracebackLoadAttr(tb, "tb_frame")
		if err != nil {
			t.Fatalf("tb_frame: %v", err)
		}
		code, err := frameLoadAttr(frameObj.(*frameObject), "f_code")
		if err != nil {
			t.Fatalf("f_code: %v", err)
		}
		coName, _ := codeLoadAttr(code.(*codeObject), "co_name")
		if Str(coName) != w.name {
			t.Errorf("node %d co_name = %q, want %q", i, Str(coName), w.name)
		}
		// f_lineno tracks the same live line tb_lineno reports.
		flineno, _ := frameLoadAttr(frameObj.(*frameObject), "f_lineno")
		lineno, err := tracebackLoadAttr(tb, "tb_lineno")
		if err != nil {
			t.Fatalf("tb_lineno: %v", err)
		}
		if Str(lineno) != w.lineno || Str(flineno) != w.lineno {
			t.Errorf("node %d tb_lineno/f_lineno = %v/%v, want %s", i, Str(lineno), Str(flineno), w.lineno)
		}
		// tb_lasti has no compiled equivalent, so it reads back -1.
		lasti, _ := tracebackLoadAttr(tb, "tb_lasti")
		if Str(lasti) != "-1" {
			t.Errorf("node %d tb_lasti = %v, want -1", i, Str(lasti))
		}
		next, err := tracebackLoadAttr(tb, "tb_next")
		if err != nil {
			t.Fatalf("tb_next: %v", err)
		}
		if i == len(wants)-1 {
			if next != None {
				t.Errorf("last node tb_next = %v, want None", next)
			}
			tb = nil
		} else {
			tb = next.(*tracebackObject)
		}
	}
	if r := tracebackRepr(tracebackFromFrames(frames)); !strings.HasPrefix(r, "<traceback object at 0x") {
		t.Errorf("repr = %q, want <traceback object at 0x...>", r)
	}
}

func TestExcTracebackProjection(t *testing.T) {
	// A fresh exception with no frames reads back None. Once it carries frames,
	// __traceback__ projects a traceback object, and repeated reads hand back the
	// same cached object the way CPython caches one tb per exception.
	fresh := &Exception{Kind: "ValueError", Args: []Object{NewStr("x")}}
	got, err := excLoadAttr(fresh, "__traceback__")
	if err != nil {
		t.Fatalf("fresh __traceback__: %v", err)
	}
	if got != None {
		t.Errorf("fresh __traceback__ = %v, want None", got)
	}

	raised := &Exception{Kind: "ValueError", Args: []Object{NewStr("x")}, Frames: []Frame{{File: "m.py", Line: 2, Func: "f"}}}
	first, err := excLoadAttr(raised, "__traceback__")
	if err != nil || first == None {
		t.Fatalf("raised __traceback__ = %v, %v, want a traceback", first, err)
	}
	if _, ok := first.(*tracebackObject); !ok {
		t.Fatalf("raised __traceback__ = %T, want *tracebackObject", first)
	}
	second, _ := excLoadAttr(raised, "__traceback__")
	if first != second {
		t.Error("repeated __traceback__ reads returned different objects, want the cached one")
	}

	// with_traceback stores an explicit value that wins over the projection,
	// including None, so unittest's exc.with_traceback(None) reads back None.
	if _, err := excMethod(raised, "with_traceback", []Object{None}); err != nil {
		t.Fatalf("with_traceback(None): %v", err)
	}
	override, _ := excLoadAttr(raised, "__traceback__")
	if override != None {
		t.Errorf("__traceback__ after with_traceback(None) = %v, want None", override)
	}
}
