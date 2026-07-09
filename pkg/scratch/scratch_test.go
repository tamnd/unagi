package scratch

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// fixedNow is an arbitrary reference time. The tests never call the real clock,
// so ages are all measured against this constant.
var fixedNow = time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)

func TestShouldReclaimBase(t *testing.T) {
	dead := func(int) bool { return false }
	alive := func(int) bool { return true }
	const self = 4242

	cases := []struct {
		name    string
		dir     string
		mtime   time.Time
		isAlive func(int) bool
		want    bool
	}{
		{"dead pid base is reclaimed", "unagi-scratch-1001-abcd", fixedNow, dead, true},
		{"live pid base is kept", "unagi-scratch-1001-abcd", fixedNow, alive, false},
		{"own base is kept even fresh", "unagi-scratch-4242-abcd", fixedNow, dead, false},
		{"own base is kept when reported alive", "unagi-scratch-4242-abcd", fixedNow, alive, false},
		{"malformed base falls back to age, fresh kept", "unagi-scratch-notapid", fixedNow, dead, false},
		{"malformed base falls back to age, old reclaimed", "unagi-scratch-notapid", fixedNow.Add(-3 * time.Hour), dead, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := shouldReclaim(c.dir, c.mtime, self, fixedNow, c.isAlive)
			if got != c.want {
				t.Fatalf("shouldReclaim(%q, alive=%v) = %v, want %v", c.dir, c.isAlive(0), got, c.want)
			}
		})
	}
}

func TestShouldReclaimLegacy(t *testing.T) {
	dead := func(int) bool { return false }
	const self = 4242

	cases := []struct {
		name  string
		dir   string
		mtime time.Time
		want  bool
	}{
		{"fresh gen dir is kept", "unagi-gen-abcd", fixedNow, false},
		{"old gen dir is reclaimed", "unagi-gen-abcd", fixedNow.Add(-3 * time.Hour), true},
		{"old conf dir is reclaimed", "unagi-conf-abcd", fixedNow.Add(-3 * time.Hour), true},
		{"old run dir is reclaimed", "unagi-run-abcd", fixedNow.Add(-3 * time.Hour), true},
		{"unrelated dir is never touched", "go-build123", fixedNow.Add(-100 * time.Hour), false},
		{"another unrelated dir is never touched", "decompressed-browser", fixedNow.Add(-100 * time.Hour), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := shouldReclaim(c.dir, c.mtime, self, fixedNow, dead)
			if got != c.want {
				t.Fatalf("shouldReclaim(%q) = %v, want %v", c.dir, got, c.want)
			}
		})
	}
}

func TestBasePID(t *testing.T) {
	cases := []struct {
		name    string
		wantPID int
		wantOK  bool
	}{
		{"unagi-scratch-1234-abcd", 1234, true},
		{"unagi-scratch-7-x", 7, true},
		{"unagi-scratch-abc-x", 0, false},
		{"unagi-scratch-", 0, false},
		{"unagi-scratch-1234", 0, false},
	}
	for _, c := range cases {
		pid, ok := basePID(c.name)
		if pid != c.wantPID || ok != c.wantOK {
			t.Fatalf("basePID(%q) = (%d, %v), want (%d, %v)", c.name, pid, ok, c.wantPID, c.wantOK)
		}
	}
}

// TestReclaimSweepsOnlyAbandoned builds a temp root by hand, populates it with a
// mix of live, dead, own, legacy, and unrelated directories, and checks reclaim
// removes exactly the abandoned scratch and leaves everything else in place. It
// touches only the small tree it creates under t.TempDir, so it stays disk-light.
func TestReclaimSweepsOnlyAbandoned(t *testing.T) {
	root := t.TempDir()
	const self = 4242
	old := fixedNow.Add(-3 * time.Hour)

	mk := func(name string, mtime time.Time) {
		p := filepath.Join(root, name)
		if err := os.Mkdir(p, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(p, mtime, mtime); err != nil {
			t.Fatal(err)
		}
	}

	mk("unagi-scratch-1001-dead", fixedNow) // dead pid -> removed
	mk("unagi-scratch-2002-live", fixedNow) // live pid -> kept
	mk("unagi-scratch-4242-self", fixedNow) // own pid -> kept
	mk("unagi-gen-old", old)                // old legacy -> removed
	mk("unagi-gen-fresh", fixedNow)         // fresh legacy -> kept
	mk("go-build-unrelated", old)           // unrelated -> kept

	alivePIDs := map[int]bool{2002: true}
	isAlive := func(pid int) bool { return alivePIDs[pid] }

	removed := reclaim(root, self, fixedNow, isAlive)
	if removed != 2 {
		t.Fatalf("reclaim removed %d, want 2", removed)
	}

	stillThere := func(name string) bool {
		_, err := os.Stat(filepath.Join(root, name))
		return err == nil
	}
	for _, name := range []string{"unagi-scratch-2002-live", "unagi-scratch-4242-self", "unagi-gen-fresh", "go-build-unrelated"} {
		if !stillThere(name) {
			t.Errorf("%q was removed but should have been kept", name)
		}
	}
	for _, name := range []string{"unagi-scratch-1001-dead", "unagi-gen-old"} {
		if stillThere(name) {
			t.Errorf("%q was kept but should have been removed", name)
		}
	}
}

// TestScopeBoundsAndRestores checks the full Scope lifecycle on a small tree: it
// points TMPDIR at a fresh base under the prior temp root, and its cleanup both
// restores TMPDIR and removes the base, so the run leaves nothing behind.
func TestScopeBoundsAndRestores(t *testing.T) {
	root := t.TempDir()
	t.Setenv("TMPDIR", root)

	cleanup, err := Scope()
	if err != nil {
		t.Fatal(err)
	}
	base := os.Getenv("TMPDIR")
	if filepath.Dir(base) != root {
		t.Fatalf("Scope base %q is not under root %q", base, root)
	}
	if _, err := os.Stat(base); err != nil {
		t.Fatalf("Scope base does not exist: %v", err)
	}

	cleanup()
	if got := os.Getenv("TMPDIR"); got != root {
		t.Fatalf("cleanup left TMPDIR=%q, want %q", got, root)
	}
	if _, err := os.Stat(base); !os.IsNotExist(err) {
		t.Fatalf("cleanup left base behind: stat err = %v", err)
	}
}
