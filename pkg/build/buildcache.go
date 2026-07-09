package build

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"sync"
)

// This file adds a content-addressed binary cache in front of the Go toolchain.
//
// Why it exists. The conformance suite builds every fixture three times, once
// per tier (auto, forced-static, forced-boxed). For the many fixtures that carry
// no statically provable unit the three tiers emit byte-identical Go, so the same
// program links two or three times, and linking a binary that embeds the runtime
// is the slow, memory-heavy step of the pipeline. Keyed on the exact bytes that
// determine the binary, a build that has already been linked in this run is
// served by copying the cached executable instead of running the linker again.
//
// Correctness. The key covers everything the linked binary depends on: the target
// platform, the Go toolchain version, the fingerprint of the runtime source the
// generated module vendors, and the emitted files themselves. The cache lives for
// one process only (its directory is created under the temp root the test harness
// scopes and reaps per run), so the runtime never changes underneath a cached
// entry, and a stale hit that would serve a wrong binary cannot happen. A wrong
// answer served fast is still wrong, so the cache is keyed to never serve one.
//
// Safety. The store is bounded: a size cap evicts the least recently used
// binaries once the cache would grow past it, so a large corpus can never fill
// the disk. It is opt-out with UNAGI_BUILD_CACHE=off and the cap is tunable with
// UNAGI_BUILD_CACHE_MB. Because it turns repeated links into file copies it also
// lowers peak linker memory, the other half of the disk-and-OOM budget the
// suite runs under.

// binCache is the process-wide build cache. Its directory is created lazily on
// the first miss so a run that never builds leaves nothing behind.
var binCache = &buildCache{
	entries: map[string]*cacheEntry{},
	keyLock: map[string]*sync.Mutex{},
}

// cacheEntry is one linked binary in the store.
type cacheEntry struct {
	path string // canonical file inside the cache directory
	size int64
	used uint64 // last-use tick, for least-recently-used eviction
}

type buildCache struct {
	mu      sync.Mutex
	dir     string
	dirErr  error
	made    bool
	entries map[string]*cacheEntry
	keyLock map[string]*sync.Mutex // per-key locks serialize identical builds
	tick    uint64
	bytes   int64
}

// enabled reports whether the cache is active. UNAGI_BUILD_CACHE=off turns it
// off, which restores the plain build-into-out path for debugging.
func cacheEnabled() bool {
	return os.Getenv("UNAGI_BUILD_CACHE") != "off"
}

// capOverride lets a test set the cap in bytes directly, below the one-megabyte
// granularity UNAGI_BUILD_CACHE_MB allows, so eviction is exercised with tiny
// payloads. Zero means unset; production never touches it.
var capOverride int64

// capBytes is the cache size ceiling. A test override wins, then
// UNAGI_BUILD_CACHE_MB, then the default, and a non-positive value removes the
// cap.
func capBytes() int64 {
	if capOverride != 0 {
		return capOverride
	}
	if s := os.Getenv("UNAGI_BUILD_CACHE_MB"); s != "" {
		if n, err := strconv.ParseInt(s, 10, 64); err == nil {
			return n << 20
		}
	}
	return 2 << 30 // 2 GiB
}

// buildBinary produces the binary for the module in genDir at out. With the cache
// on it links the program once per distinct key and copies the cached executable
// on later hits; with it off it links straight into out the way a plain build
// would. link is the toolchain call, passed in so this file does not import the
// build flags it belongs to.
func (c *buildCache) buildBinary(genDir, out string, link func(dst string) error) error {
	if !cacheEnabled() {
		return link(out)
	}
	key, err := c.key(genDir)
	if err != nil {
		// A key we cannot compute is not a reason to fail the build; fall back to
		// linking directly, so the cache is a pure optimization.
		return link(out)
	}

	kl := c.lockFor(key)
	kl.Lock()
	defer kl.Unlock()

	if path, ok := c.hit(key); ok {
		return copyExecutable(path, out)
	}

	dir, err := c.ensureDir()
	if err != nil {
		return link(out)
	}
	canonical := filepath.Join(dir, key)
	if err := link(canonical); err != nil {
		return err
	}
	c.store(key, canonical)
	return copyExecutable(canonical, out)
}

// hit returns a cached binary's path if the key is present and the file still
// exists, bumping its use tick so eviction keeps the hot set.
func (c *buildCache) hit(key string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok {
		return "", false
	}
	if _, err := os.Stat(e.path); err != nil {
		// Evicted or removed out from under us; drop the record and miss.
		c.bytes -= e.size
		delete(c.entries, key)
		return "", false
	}
	c.tick++
	e.used = c.tick
	return e.path, true
}

// store records a freshly linked binary and evicts the least recently used
// entries if the cache has grown past the cap.
func (c *buildCache) store(key, path string) {
	info, err := os.Stat(path)
	if err != nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.tick++
	c.entries[key] = &cacheEntry{path: path, size: info.Size(), used: c.tick}
	c.bytes += info.Size()
	c.evictLocked(key)
}

// evictLocked removes least recently used binaries until the cache is under the
// cap. keep is the entry just stored, never evicted even if the cap is smaller
// than a single binary, so the current build always has its file.
func (c *buildCache) evictLocked(keep string) {
	limit := capBytes()
	if limit <= 0 || c.bytes <= limit {
		return
	}
	keys := make([]string, 0, len(c.entries))
	for k := range c.entries {
		if k != keep {
			keys = append(keys, k)
		}
	}
	sort.Slice(keys, func(i, j int) bool {
		return c.entries[keys[i]].used < c.entries[keys[j]].used
	})
	for _, k := range keys {
		if c.bytes <= limit {
			return
		}
		e := c.entries[k]
		if os.Remove(e.path) == nil {
			c.bytes -= e.size
			delete(c.entries, k)
		}
	}
}

// lockFor returns the per-key lock that serializes concurrent builds of the same
// program, so two parallel fixtures with identical output link once, not twice.
func (c *buildCache) lockFor(key string) *sync.Mutex {
	c.mu.Lock()
	defer c.mu.Unlock()
	l, ok := c.keyLock[key]
	if !ok {
		l = &sync.Mutex{}
		c.keyLock[key] = l
	}
	return l
}

// ensureDir creates the cache directory once, under the temp root. In a test run
// the harness points the temp root at a per-run base it deletes on exit, so the
// cache lives and dies with the run and never accumulates across runs.
func (c *buildCache) ensureDir() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.made {
		return c.dir, c.dirErr
	}
	c.made = true
	c.dir, c.dirErr = os.MkdirTemp("", "unagi-buildcache-")
	return c.dir, c.dirErr
}

// key hashes everything the linked binary depends on: platform, toolchain, the
// vendored runtime source fingerprint, and the emitted files. The runtime copy
// under unagi-src is folded in once through the fingerprint rather than rehashed
// per build, so the per-build cost is hashing only the small emitted files.
func (c *buildCache) key(genDir string) (string, error) {
	h := sha256.New()
	_, _ = fmt.Fprintf(h, "unagi-bincache-v1\n%s\n%s\n%s\n%s\n",
		runtime.GOOS, runtime.GOARCH, os.Getenv("CGO_ENABLED"), runtime.Version())
	fp, err := runtimeFingerprint()
	if err != nil {
		return "", err
	}
	_, _ = io.WriteString(h, fp)
	if err := hashEmitted(h, genDir); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// hashEmitted folds every generated file under genDir into h in a stable order,
// skipping the vendored runtime copy, which the fingerprint already covers.
func hashEmitted(h io.Writer, genDir string) error {
	var files []string
	slim := filepath.Join(genDir, "unagi-src")
	err := filepath.Walk(genDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if path == slim {
				return filepath.SkipDir
			}
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return err
	}
	sort.Strings(files)
	for _, path := range files {
		rel, err := filepath.Rel(genDir, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(h, "%s\n%d\n", rel, len(data))
		if _, err := h.Write(data); err != nil {
			return err
		}
	}
	return nil
}

// runtimeFingerprint hashes the runtime packages the generated module vendors.
// It is computed once per process because every build in a run vendors the same
// source, so a change to the runtime invalidates every cached binary at once.
var (
	fpOnce sync.Once
	fpVal  string
	fpErr  error
)

func runtimeFingerprint() (string, error) {
	fpOnce.Do(func() {
		src, err := sourceDir()
		if err != nil {
			fpErr = err
			return
		}
		h := sha256.New()
		for _, pkg := range []string{"objects", "runtime", "sre"} {
			dir := filepath.Join(src, "pkg", pkg)
			entries, err := os.ReadDir(dir)
			if err != nil {
				fpErr = err
				return
			}
			names := make([]string, 0, len(entries))
			for _, e := range entries {
				n := e.Name()
				if e.IsDir() || !hasSuffix(n, ".go") || hasSuffix(n, "_test.go") {
					continue
				}
				names = append(names, n)
			}
			sort.Strings(names)
			for _, n := range names {
				data, err := os.ReadFile(filepath.Join(dir, n))
				if err != nil {
					fpErr = err
					return
				}
				_, _ = fmt.Fprintf(h, "%s/%s\n%d\n", pkg, n, len(data))
				_, _ = h.Write(data)
			}
		}
		fpVal = hex.EncodeToString(h.Sum(nil))
	})
	return fpVal, fpErr
}

func hasSuffix(s, suf string) bool {
	return len(s) >= len(suf) && s[len(s)-len(suf):] == suf
}

// copyExecutable writes src to dst with the executable bit, through a temp file
// renamed into place so a reader never sees a half-written binary.
func copyExecutable(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".unagi-bin-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := io.Copy(tmp, in); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, 0o755); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, dst)
}

// CleanupCache removes the cache directory and resets the store. The test
// harness calls it on the way out; the per-run temp root also reaps the
// directory, so this is belt and suspenders for a clean shutdown.
func CleanupCache() {
	binCache.mu.Lock()
	defer binCache.mu.Unlock()
	if binCache.dir != "" {
		_ = os.RemoveAll(binCache.dir)
	}
	binCache.dir = ""
	binCache.made = false
	binCache.dirErr = nil
	binCache.entries = map[string]*cacheEntry{}
	binCache.keyLock = map[string]*sync.Mutex{}
	binCache.bytes = 0
	binCache.tick = 0
}
