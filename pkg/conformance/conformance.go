// Package conformance is the differential harness of doc 19: it runs a
// fixture through the compiled subject and the CPython oracle, normalizes
// both outputs, and judges the comparison. The runner is deliberately dumb;
// the intelligence sits in the traceback parser and the normalizer.
package conformance

import "time"

// Verdict is the judged status of one fixture run.
type Verdict int

const (
	Pass Verdict = iota
	Fail
	DivergentOK
	BuildError
	GoldenStale // nightly only: golden disagrees with live oracle
)

func (v Verdict) String() string {
	switch v {
	case Pass:
		return "PASS"
	case Fail:
		return "FAIL"
	case DivergentOK:
		return "DIVERGENT-OK"
	case BuildError:
		return "BUILD-ERROR"
	case GoldenStale:
		return "GOLDEN-STALE"
	}
	return "UNKNOWN"
}

// Channel names one of the four compared output channels.
type Channel int

const (
	ChanStdout Channel = iota
	ChanExit
	ChanException
	ChanStderr
)

func (c Channel) String() string {
	switch c {
	case ChanStdout:
		return "stdout"
	case ChanExit:
		return "exit"
	case ChanException:
		return "exception"
	case ChanStderr:
		return "stderr"
	}
	return "unknown"
}

// Outcome is what one pipeline (oracle or subject) produced.
type Outcome struct {
	Stdout []byte
	Stderr []byte // traceback region excised, unless RawTraceback
	Exit   int
	// Exception is the parsed traceback surface, nil when the program
	// exited without one.
	Exception *ExceptionSurface
	// RawTraceback marks an exception-group traceback kept verbatim in
	// Stderr because the parser does not build surfaces for groups yet;
	// group structure compares textually until except* lands in M2.
	RawTraceback bool
	Wall         time.Duration
	TimedOut     bool
}

// Result is the judged comparison for one fixture.
type Result struct {
	Fixture  string
	Verdict  Verdict
	Diffs    []Diff   // empty on Pass
	Ledgered []string // ledger ids that absorbed a diff
	Skipped  bool     // fixture.toml [skip]; Verdict is not meaningful
	SkipWhy  string
	BuildErr string // BuildError only
	Oracle   Outcome
	Subject  Outcome
}

// Diff is one observed difference on one channel.
type Diff struct {
	Channel Channel
	// For stdout/stderr: the first differing line and its index.
	// For exit: the two integers.
	// For exception: a path like "frames[2].Line" into the surface.
	Where  string
	Oracle string
	Got    string
}

// ExceptionSurface is the structural form of an uncaught exception: what a
// traceback shows, minus presentation.
type ExceptionSurface struct {
	Type    string  // "ZeroDivisionError", fully qualified for non-builtins
	Message string  // str(exc); PEP 678 notes fold in as extra lines
	Frames  []Frame // outermost first, as printed
	Cause   *ExceptionSurface
	Context *ExceptionSurface
}

// Frame is one traceback frame.
type Frame struct {
	File string // path relative to the fixture root
	Line int    // 1-based line in the Python source
	Name string // the code object name, "<module>" at top level
}
