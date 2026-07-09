package build

import (
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
)

// genDirWith writes a minimal generated module under a fresh directory: a
// main.go with the given body and the vendored runtime marker the key walk
// skips. It is enough for the cache key to hash something stable and distinct.
func genDirWith(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	// A runtime copy the key must skip; putting a file here proves the skip.
	slim := filepath.Join(dir, "unagi-src")
	if err := os.MkdirAll(slim, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(slim, "go.mod"), []byte("module x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// fakeLink writes a fixed payload to dst and counts how often it runs, standing
// in for the Go toolchain so the cache logic is tested without a real link.
func fakeLink(payload string, calls *int32) func(string) error {
	return func(dst string) error {
		atomic.AddInt32(calls, 1)
		return os.WriteFile(dst, []byte(payload), 0o755)
	}
}

// TestBuildCacheReusesLink proves the second build of an identical module copies
// the cached binary instead of linking again.
func TestBuildCacheReusesLink(t *testing.T) {
	CleanupCache()
	defer CleanupCache()
	gen := genDirWith(t, "package main\nfunc main(){}\n")
	var calls int32
	link := fakeLink("BINARY-ONE", &calls)

	out1 := filepath.Join(t.TempDir(), "a")
	if err := binCache.buildBinary(gen, out1, link); err != nil {
		t.Fatal(err)
	}
	out2 := filepath.Join(t.TempDir(), "b")
	if err := binCache.buildBinary(gen, out2, link); err != nil {
		t.Fatal(err)
	}

	if calls != 1 {
		t.Fatalf("identical modules should link once, linked %d times", calls)
	}
	for _, out := range []string{out1, out2} {
		data, err := os.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "BINARY-ONE" {
			t.Fatalf("cached binary content wrong at %s: %q", out, data)
		}
		if info, _ := os.Stat(out); info.Mode()&0o100 == 0 {
			t.Fatalf("cached binary at %s is not executable", out)
		}
	}
}

// TestBuildCacheDistinctModulesLinkSeparately proves a different emitted module
// is a different key, so it links on its own rather than serving a stale hit.
func TestBuildCacheDistinctModulesLinkSeparately(t *testing.T) {
	CleanupCache()
	defer CleanupCache()
	var calls int32
	link := fakeLink("PAYLOAD", &calls)

	genA := genDirWith(t, "package main // A\nfunc main(){}\n")
	genB := genDirWith(t, "package main // B\nfunc main(){}\n")
	if err := binCache.buildBinary(genA, filepath.Join(t.TempDir(), "a"), link); err != nil {
		t.Fatal(err)
	}
	if err := binCache.buildBinary(genB, filepath.Join(t.TempDir(), "b"), link); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("two distinct modules should link twice, linked %d times", calls)
	}
}

// TestBuildCacheDisabled proves UNAGI_BUILD_CACHE=off restores the plain
// link-into-out path with no reuse.
func TestBuildCacheDisabled(t *testing.T) {
	CleanupCache()
	defer CleanupCache()
	t.Setenv("UNAGI_BUILD_CACHE", "off")
	gen := genDirWith(t, "package main\nfunc main(){}\n")
	var calls int32
	link := fakeLink("X", &calls)
	for i := 0; i < 3; i++ {
		if err := binCache.buildBinary(gen, filepath.Join(t.TempDir(), "o"), link); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 3 {
		t.Fatalf("with the cache off every build should link, linked %d times", calls)
	}
}

// TestBuildCacheEvictsUnderCap proves the cache stays under its size cap by
// evicting the least recently used binary, which then relinks on next use.
func TestBuildCacheEvictsUnderCap(t *testing.T) {
	CleanupCache()
	defer CleanupCache()

	payload := "0123456789" // 10 bytes each
	genA := genDirWith(t, "package main //A\n")
	genB := genDirWith(t, "package main //B\n")
	var calls int32
	link := fakeLink(payload, &calls)

	// Cap at 15 bytes: holds one 10-byte binary, a second store evicts the first.
	oldCap := capOverride
	capOverride = 15
	defer func() { capOverride = oldCap }()

	if err := binCache.buildBinary(genA, filepath.Join(t.TempDir(), "a1"), link); err != nil {
		t.Fatal(err)
	}
	if err := binCache.buildBinary(genB, filepath.Join(t.TempDir(), "b1"), link); err != nil {
		t.Fatal(err)
	}
	// genA was evicted by genB's store, so building it again relinks.
	if err := binCache.buildBinary(genA, filepath.Join(t.TempDir(), "a2"), link); err != nil {
		t.Fatal(err)
	}
	if calls != 3 {
		t.Fatalf("an evicted module should relink: want 3 links, got %d", calls)
	}
}

// TestBuildCacheConcurrentSameKey proves parallel builds of one module link once
// under the per-key lock and every caller still gets the binary.
func TestBuildCacheConcurrentSameKey(t *testing.T) {
	CleanupCache()
	defer CleanupCache()
	gen := genDirWith(t, "package main\nfunc main(){}\n")
	var calls int32
	link := fakeLink("ONCE", &calls)

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			out := filepath.Join(t.TempDir(), "o")
			if err := binCache.buildBinary(gen, out, link); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	if calls != 1 {
		t.Fatalf("concurrent identical builds should link once, linked %d times", calls)
	}
}
