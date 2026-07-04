// The judge: channel-by-channel comparison of two normalized Outcomes.
// Stdout and stderr compare as raw bytes with no whitespace forgiveness,
// exit as an integer, and the exception surface structurally, message text
// included, because CPython 3.14 error wording is part of the contract.
package conformance

import (
	"fmt"
	"strings"
)

// compare judges subject against oracle. When every diff is covered by a
// declared ledger id the verdict is DivergentOK; per-channel confinement
// waits for a channels field on ledger entries (plan/19 note).
func compare(oracle, subject Outcome, ledgerIDs []string) Result {
	var diffs []Diff
	diffs = append(diffs, diffBytes(ChanStdout, oracle.Stdout, subject.Stdout)...)
	if oracle.Exit != subject.Exit {
		diffs = append(diffs, Diff{
			Channel: ChanExit,
			Where:   fmt.Sprintf("%d vs %d", oracle.Exit, subject.Exit),
			Oracle:  fmt.Sprint(oracle.Exit),
			Got:     fmt.Sprint(subject.Exit),
		})
	}
	diffs = append(diffs, diffSurface("", oracle.Exception, subject.Exception)...)
	diffs = append(diffs, diffBytes(ChanStderr, oracle.Stderr, subject.Stderr)...)

	res := Result{Oracle: oracle, Subject: subject, Diffs: diffs}
	switch {
	case len(diffs) == 0:
		res.Verdict = Pass
	case len(ledgerIDs) > 0:
		res.Verdict = DivergentOK
		res.Ledgered = ledgerIDs
	default:
		res.Verdict = Fail
	}
	return res
}

// diffBytes reports the first differing line of a byte channel.
func diffBytes(ch Channel, want, got []byte) []Diff {
	if string(want) == string(got) {
		return nil
	}
	wl := strings.Split(string(want), "\n")
	gl := strings.Split(string(got), "\n")
	for i := 0; i < len(wl) || i < len(gl); i++ {
		var w, g string
		if i < len(wl) {
			w = wl[i]
		}
		if i < len(gl) {
			g = gl[i]
		}
		if w != g {
			return []Diff{{Channel: ch, Where: fmt.Sprintf("line %d", i+1), Oracle: w, Got: g}}
		}
	}
	return []Diff{{Channel: ch, Where: "length", Oracle: fmt.Sprint(len(want)), Got: fmt.Sprint(len(got))}}
}

// diffSurface compares exception surfaces structurally and recursively:
// Type and Message exactly, every frame on File, Line, and Name, and the
// Cause and Context chains with the same rules.
func diffSurface(path string, want, got *ExceptionSurface) []Diff {
	at := func(field string) string {
		if path == "" {
			return field
		}
		return path + "." + field
	}
	switch {
	case want == nil && got == nil:
		return nil
	case want == nil:
		return []Diff{{Channel: ChanException, Where: at("presence"), Oracle: "no exception", Got: got.Type}}
	case got == nil:
		return []Diff{{Channel: ChanException, Where: at("presence"), Oracle: want.Type, Got: "no exception"}}
	}
	var diffs []Diff
	if want.Type != got.Type {
		diffs = append(diffs, Diff{Channel: ChanException, Where: at("type"), Oracle: want.Type, Got: got.Type})
	}
	if want.Message != got.Message {
		diffs = append(diffs, Diff{Channel: ChanException, Where: at("message"), Oracle: want.Message, Got: got.Message})
	}
	if len(want.Frames) != len(got.Frames) {
		diffs = append(diffs, Diff{
			Channel: ChanException,
			Where:   at("frames"),
			Oracle:  fmt.Sprintf("%d frames", len(want.Frames)),
			Got:     fmt.Sprintf("%d frames", len(got.Frames)),
		})
	} else {
		for i := range want.Frames {
			if want.Frames[i] != got.Frames[i] {
				diffs = append(diffs, Diff{
					Channel: ChanException,
					Where:   at(fmt.Sprintf("frames[%d]", i)),
					Oracle:  fmt.Sprintf("%s:%d %s", want.Frames[i].File, want.Frames[i].Line, want.Frames[i].Name),
					Got:     fmt.Sprintf("%s:%d %s", got.Frames[i].File, got.Frames[i].Line, got.Frames[i].Name),
				})
			}
		}
	}
	diffs = append(diffs, diffSurface(at("cause"), want.Cause, got.Cause)...)
	diffs = append(diffs, diffSurface(at("context"), want.Context, got.Context)...)
	return diffs
}
