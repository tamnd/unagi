package conformance

import (
	"os"
	"path/filepath"
	"testing"
)

// TestCompareHoldsRaisingChannels is doc 10 item 31: a raising fixture is judged
// on every channel a traceback lands in, not stdout alone. It proves the judge
// flags a drift in the exception type, in the message text (CPython 3.14 wording
// is part of the contract), and in the raw stderr, and that two identical
// raising outcomes pass. Frame-shape drift is already covered by
// TestCompareCatchesSurfaceDrift.
func TestCompareHoldsRaisingChannels(t *testing.T) {
	base := Outcome{Exit: 1, Exception: &ExceptionSurface{
		Type:    "ZeroDivisionError",
		Message: "division by zero",
		Frames:  []Frame{{"main.py", 2, "<module>"}},
	}}
	if res := compare(base, base, nil); res.Verdict != Pass {
		t.Fatalf("identical raising outcomes should pass, got %+v", res)
	}

	cases := []struct {
		name    string
		got     Outcome
		channel Channel
	}{
		{
			"type drift",
			Outcome{Exit: 1, Exception: &ExceptionSurface{Type: "ValueError", Message: "division by zero", Frames: base.Exception.Frames}},
			ChanException,
		},
		{
			"message drift",
			Outcome{Exit: 1, Exception: &ExceptionSurface{Type: "ZeroDivisionError", Message: "attempt to divide", Frames: base.Exception.Frames}},
			ChanException,
		},
		{
			"stderr drift",
			Outcome{Exit: 1, Exception: base.Exception, Stderr: []byte("extra traceback noise")},
			ChanStderr,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res := compare(base, c.got, nil)
			if res.Verdict != Fail {
				t.Fatalf("%s should fail, got %v", c.name, res.Verdict)
			}
			found := false
			for _, d := range res.Diffs {
				if d.Channel == c.channel {
					found = true
				}
			}
			if !found {
				t.Errorf("%s produced no %s diff: %+v", c.name, c.channel, res.Diffs)
			}
		})
	}
}

// TestCorpusHasRaisingFixtures asserts the corpus actually exercises the raising
// path the judge above guards: at least one fixture whose recorded oracle
// carries a parsed exception surface, with a ZeroDivisionError representative,
// so the full-channel assertion is not vacuous. It reads goldens off disk and
// never builds, so it stays disk-light.
func TestCorpusHasRaisingFixtures(t *testing.T) {
	fixtures, err := Discover(filepath.Join("..", "..", "conformance", "fixtures"))
	if err != nil {
		t.Fatal(err)
	}
	raising := 0
	types := map[string]bool{}
	for _, f := range fixtures {
		text, err := os.ReadFile(filepath.Join(f.Dir, "oracle.golden"))
		if err != nil {
			continue
		}
		o, err := decodeGolden(string(text))
		if err != nil {
			t.Fatalf("fixture %s: decode golden: %v", f.Name, err)
		}
		if o.Exception != nil {
			raising++
			types[o.Exception.Type] = true
		}
	}
	if raising == 0 {
		t.Fatal("no fixture carries an exception surface; the raising-channel judge is untested by the corpus")
	}
	if !types["ZeroDivisionError"] {
		t.Errorf("no ZeroDivisionError representative among %d raising fixtures", raising)
	}
}
