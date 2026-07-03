package conformance

import (
	"strings"
	"testing"
)

// TestGoldenWorkedExample pins the framed format to the doc 19 section 3.7
// worked example byte for byte.
func TestGoldenWorkedExample(t *testing.T) {
	o := Outcome{
		Exit:   1,
		Stdout: []byte("start\n"),
		Exception: &ExceptionSurface{
			Type:    "RuntimeError",
			Message: "bad record",
			Frames:  []Frame{{"main.py", 8, "<module>"}, {"main.py", 5, "parse"}},
			Cause: &ExceptionSurface{
				Type:    "ValueError",
				Message: "invalid literal for int() with base 10: 'x'",
				Frames:  []Frame{{"main.py", 3, "parse"}},
			},
		},
	}
	want := strings.Join([]string{
		"== exit: 1",
		"== stdout:",
		"start",
		"== exception:",
		"RuntimeError: bad record",
		"  main.py:8 <module>",
		"  main.py:5 parse",
		"cause:",
		"ValueError: invalid literal for int() with base 10: 'x'",
		"  main.py:3 parse",
		"== stderr:",
		"",
	}, "\n")
	if got := encodeGolden(o); got != want {
		t.Errorf("encode mismatch\n--- got ---\n%s--- want ---\n%s", got, want)
	}
}

func TestGoldenRoundTrip(t *testing.T) {
	cases := []Outcome{
		{Exit: 0, Stdout: []byte("hello\n")},
		{Exit: 0},
		{Exit: 3, Stdout: []byte("no newline")},
		{Exit: 1, Stderr: []byte("residual warning\n"), Exception: &ExceptionSurface{
			Type: "KeyboardInterrupt", Frames: []Frame{{"main.py", 1, "<module>"}},
		}},
		{Exit: 1, Exception: &ExceptionSurface{
			Type:    "ValueError",
			Message: "base\nnote one\nnote two",
			Frames:  []Frame{{"main.py", 4, "<module>"}},
			Context: &ExceptionSurface{Type: "KeyError", Message: "'k'", Frames: []Frame{{"main.py", 2, "<module>"}}},
		}},
	}
	for i, o := range cases {
		text := encodeGolden(o)
		back, err := decodeGolden(text)
		if err != nil {
			t.Errorf("case %d: decode: %v", i, err)
			continue
		}
		if res := compare(o, back, nil); res.Verdict != Pass {
			t.Errorf("case %d: round trip diverged: %+v\nframed:\n%s", i, res.Diffs, text)
		}
	}
}

// TestGoldenGroupInStderr checks the group concession: no exception
// section, traceback text kept in stderr, RawTraceback set on decode.
func TestGoldenGroupInStderr(t *testing.T) {
	tb := "  + Exception Group Traceback (most recent call last):\n  |   File \"main.py\", line 1, in <module>\n  | ExceptionGroup: boom (2 sub-exceptions)\n"
	o := Outcome{Exit: 1, Stderr: []byte(tb), RawTraceback: true}
	back, err := decodeGolden(encodeGolden(o))
	if err != nil {
		t.Fatal(err)
	}
	if !back.RawTraceback {
		t.Error("RawTraceback not inferred from stderr traceback text")
	}
	if string(back.Stderr) != tb {
		t.Errorf("stderr = %q", back.Stderr)
	}
}

func TestCompareCatchesSurfaceDrift(t *testing.T) {
	base := Outcome{Exit: 1, Exception: &ExceptionSurface{
		Type: "ValueError", Message: "x", Frames: []Frame{{"main.py", 3, "<module>"}},
	}}
	drifted := Outcome{Exit: 1, Exception: &ExceptionSurface{
		Type: "ValueError", Message: "x", Frames: []Frame{{"main.py", 4, "<module>"}},
	}}
	res := compare(base, drifted, nil)
	if res.Verdict != Fail || len(res.Diffs) != 1 || res.Diffs[0].Where != "frames[0]" {
		t.Errorf("result = %+v", res)
	}
	ok := compare(base, drifted, []string{"CL-01"})
	if ok.Verdict != DivergentOK {
		t.Errorf("ledgered verdict = %v", ok.Verdict)
	}
}
