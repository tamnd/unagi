package conformance

import (
	"os"
	"path/filepath"
	"testing"
)

// sample loads a recorded CPython stderr sample, normalized the way the
// runner normalizes oracle stderr. The samples were recorded from python3.14
// running under /private/tmp/tbsamples, so that path is the N1 root.
func sample(t *testing.T, name string) (residual, region string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "tracebacks", name+".txt"))
	if err != nil {
		t.Fatal(err)
	}
	s := normalize(stripCarets(stripOracleNoise(string(data))), "/private/tmp/tbsamples")
	return splitStderr(s)
}

func parseSample(t *testing.T, name string) *ExceptionSurface {
	t.Helper()
	residual, region := sample(t, name)
	if residual != "" {
		t.Fatalf("unexpected residual before traceback: %q", residual)
	}
	if isGroupTraceback(region) {
		t.Fatalf("sample %s misdetected as a group traceback", name)
	}
	s, err := parseTraceback(region)
	if err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	return s
}

func TestParseSimple(t *testing.T) {
	s := parseSample(t, "simple")
	if s.Type != "ZeroDivisionError" || s.Message != "division by zero" {
		t.Errorf("head = %s: %s", s.Type, s.Message)
	}
	want := []Frame{{"simple.py", 4, "<module>"}, {"simple.py", 2, "f"}}
	if len(s.Frames) != 2 || s.Frames[0] != want[0] || s.Frames[1] != want[1] {
		t.Errorf("frames = %+v, want %+v", s.Frames, want)
	}
	if s.Cause != nil || s.Context != nil {
		t.Error("unexpected chain")
	}
}

func TestParseCauseChain(t *testing.T) {
	s := parseSample(t, "cause")
	if s.Type != "RuntimeError" || s.Message != "bad record" {
		t.Errorf("outer = %s: %s", s.Type, s.Message)
	}
	if len(s.Frames) != 2 || s.Frames[0] != (Frame{"cause.py", 6, "<module>"}) {
		t.Errorf("outer frames = %+v", s.Frames)
	}
	if s.Cause == nil || s.Cause.Type != "ValueError" {
		t.Fatalf("cause = %+v", s.Cause)
	}
	if len(s.Cause.Frames) != 1 || s.Cause.Frames[0] != (Frame{"cause.py", 3, "parse"}) {
		t.Errorf("cause frames = %+v", s.Cause.Frames)
	}
	if s.Context != nil || s.Cause.Cause != nil {
		t.Error("unexpected extra links")
	}
}

func TestParseContextChain(t *testing.T) {
	s := parseSample(t, "ctx")
	if s.Type != "TypeError" {
		t.Errorf("outer type = %s", s.Type)
	}
	if s.Context == nil || s.Context.Type != "KeyError" || s.Context.Message != "'k'" {
		t.Fatalf("context = %+v", s.Context)
	}
	if s.Cause != nil {
		t.Error("cause should be nil for an implicit chain")
	}
}

func TestParseFromNone(t *testing.T) {
	s := parseSample(t, "fromnone")
	if s.Type != "ValueError" || s.Message != "clean" {
		t.Errorf("head = %s: %s", s.Type, s.Message)
	}
	if s.Cause != nil || s.Context != nil {
		t.Error("from None must suppress the chain")
	}
}

func TestParseNotesFoldIntoMessage(t *testing.T) {
	s := parseSample(t, "notes")
	if s.Message != "base\nnote one\nnote two" {
		t.Errorf("message = %q", s.Message)
	}
}

func TestParseNoMessage(t *testing.T) {
	s := parseSample(t, "nomsg")
	if s.Type != "KeyboardInterrupt" || s.Message != "" {
		t.Errorf("head = %s: %q", s.Type, s.Message)
	}
}

// TestParseRecursionDeath checks the [Previous line repeated N more times]
// abbreviation folds into the synthetic marker frame.
func TestParseRecursionDeath(t *testing.T) {
	s := parseSample(t, "recur")
	if s.Type != "RecursionError" || s.Message != "maximum recursion depth exceeded" {
		t.Errorf("head = %s: %s", s.Type, s.Message)
	}
	last := s.Frames[len(s.Frames)-1]
	if last != (Frame{"...", 996, "[repeated]"}) {
		t.Errorf("marker frame = %+v", last)
	}
	if s.Frames[0] != (Frame{"recur.py", 3, "<module>"}) {
		t.Errorf("first frame = %+v", s.Frames[0])
	}
}

// TestSyntaxErrorStaysInStderr checks that a compile-time SyntaxError, which
// has no traceback header, is left whole in residual stderr for the byte
// compare (minus the caret line, stripped as oracle decoration).
func TestSyntaxErrorStaysInStderr(t *testing.T) {
	residual, region := sample(t, "syntax")
	if region != "" {
		t.Fatalf("syntax error misdetected as traceback region: %q", region)
	}
	want := "  File \"syn.py\", line 1\n    def f(:\nSyntaxError: invalid syntax\n"
	if residual != want {
		t.Errorf("residual = %q, want %q", residual, want)
	}
}

func TestGroupDetection(t *testing.T) {
	_, region := sample(t, "group")
	if region == "" {
		t.Fatal("no traceback region found")
	}
	if !isGroupTraceback(region) {
		t.Error("group sample not detected as a group traceback")
	}
}

func TestSplitStderrResidual(t *testing.T) {
	res, region := splitStderr("warning: something\nTraceback (most recent call last):\n  File \"main.py\", line 1, in <module>\nValueError: x\n")
	if res != "warning: something\n" {
		t.Errorf("residual = %q", res)
	}
	if region == "" {
		t.Error("region missing")
	}
	res, region = splitStderr("just noise\n")
	if res != "just noise\n" || region != "" {
		t.Errorf("no-traceback split = %q, %q", res, region)
	}
}
