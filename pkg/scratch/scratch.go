// Package scratch bounds the disk a test run leaves behind in the system temp
// directory. The build and conformance suites materialize a fresh Go module and
// link a binary per fixture under $TMPDIR; each is removed when its case
// finishes, but a run that is killed (a timeout, a Ctrl-C, an out-of-memory
// kill) never runs those deferred removals and orphans the directories. Left
// alone they accumulate across runs until the volume fills, which is exactly how
// a laptop at 99% full tips into "no space left on device" mid-link.
//
// Scope fixes this two ways at once. It confines a run's scratch to a single base
// directory it deletes on the way out, so a run that finishes (pass or fail)
// leaves nothing behind, and it reclaims the bases earlier killed runs abandoned,
// so even a run that is hard-killed is cleaned up by the next one. Steady-state
// disk use is therefore bounded to a single run's live footprint rather than
// growing without limit, and it never gates on free space or aborts a run: the
// suite always makes room by cleaning up after itself and its dead predecessors.
package scratch

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// basePrefix names the per-run base directory Scope creates. The process id is
// encoded after it so a later run can tell a base abandoned by a dead process
// from one a concurrent run is still writing to.
const basePrefix = "unagi-scratch-"

// legacyPrefixes are the scratch directory names the build and conformance
// pipelines create directly (pkg/build's unagi-gen-/unagi-run-, pkg/conformance's
// unagi-conf-). Once Scope points $TMPDIR at its base these land inside the base
// and die with it, so a bare one sitting in the shared temp root is either a
// pre-Scope orphan or a stray from the unagi CLI; either way it is reclaimed by
// age, never while it could still be in use.
var legacyPrefixes = []string{"unagi-gen-", "unagi-conf-", "unagi-run-"}

// staleAfter is how old a legacy scratch directory must be before Scope reclaims
// it. It is set well above the longest plausible build or CLI run so a directory
// that is still being written to is never swept out from under a live process.
const staleAfter = 2 * time.Hour

// Scope confines the calling process's scratch to a base directory under the
// current temp root, reclaims orphans that earlier killed runs left beside it,
// and points $TMPDIR at the base so every os.MkdirTemp("") and child `go build`
// writes underneath it. It returns a cleanup that restores $TMPDIR and removes
// the base; a test binary calls it from TestMain so the whole package's scratch
// lives and dies with the run.
func Scope() (cleanup func(), err error) {
	root := os.TempDir()
	self := os.Getpid()
	// Reclaim before creating our own base, so the sweep never has to reason
	// about the directory we are about to make.
	reclaim(root, self, time.Now(), isAlive)
	base, err := os.MkdirTemp(root, fmt.Sprintf("%s%d-", basePrefix, self))
	if err != nil {
		return nil, err
	}
	prev, had := os.LookupEnv("TMPDIR")
	if err := os.Setenv("TMPDIR", base); err != nil {
		_ = os.RemoveAll(base)
		return nil, err
	}
	return func() {
		if had {
			_ = os.Setenv("TMPDIR", prev)
		} else {
			_ = os.Unsetenv("TMPDIR")
		}
		_ = os.RemoveAll(base)
	}, nil
}

// reclaim removes the abandoned scratch directories in root and returns how many
// it removed. A per-run base is removed when the process whose id it carries is
// no longer alive (and is never removed for self); a legacy scratch directory,
// which carries no id, is removed once it is older than staleAfter. isAlive is a
// parameter so the decision is testable without spawning processes.
func reclaim(root string, self int, now time.Time, isAlive func(int) bool) int {
	entries, err := os.ReadDir(root)
	if err != nil {
		return 0
	}
	removed := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if !shouldReclaim(e.Name(), info.ModTime(), self, now, isAlive) {
			continue
		}
		if os.RemoveAll(filepath.Join(root, e.Name())) == nil {
			removed++
		}
	}
	return removed
}

// shouldReclaim decides whether a temp entry is an abandoned scratch directory
// safe to remove. A per-run base (basePrefix + pid) is reclaimable when its pid
// is neither the caller's nor a live process; a legacy scratch directory is
// reclaimable once it is older than staleAfter. Anything else is left alone.
func shouldReclaim(name string, mtime time.Time, self int, now time.Time, isAlive func(int) bool) bool {
	if strings.HasPrefix(name, basePrefix) {
		pid, ok := basePID(name)
		if !ok {
			// A base whose pid we cannot read falls back to the age rule, so a
			// malformed name still gets cleaned eventually and never at once.
			return now.Sub(mtime) >= staleAfter
		}
		return pid != self && !isAlive(pid)
	}
	for _, p := range legacyPrefixes {
		if strings.HasPrefix(name, p) {
			return now.Sub(mtime) >= staleAfter
		}
	}
	return false
}

// basePID reads the process id out of a base directory name of the form
// "unagi-scratch-<pid>-<random>".
func basePID(name string) (int, bool) {
	rest := strings.TrimPrefix(name, basePrefix)
	dash := strings.IndexByte(rest, '-')
	if dash <= 0 {
		return 0, false
	}
	pid, err := strconv.Atoi(rest[:dash])
	if err != nil {
		return 0, false
	}
	return pid, true
}

// isAlive reports whether a process with the given id currently exists. On Unix
// os.FindProcess always succeeds, so signal 0 is what actually probes the
// process: it delivers nothing but still errors when the process is gone.
func isAlive(pid int) bool {
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return p.Signal(syscall.Signal(0)) == nil
}
