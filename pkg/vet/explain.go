package vet

// explanations holds the long-form rationale for each finding code, printed by
// `unagi vet --explain CODE` in the style of go vet and rustc's --explain: what
// the pattern is, why it was safe under the GIL and is not now, and the fix.
var explanations = map[string]string{
	"UNA-THR-001": unaThr001Explain,
}

// Explain returns the long-form text for a finding code and whether the code is
// known.
func Explain(code string) (string, bool) {
	text, ok := explanations[code]
	return text, ok
}

const unaThr001Explain = `UNA-THR-001: unsynchronized read-modify-write

A read-modify-write reads a value, changes it, and stores the result back, which
is three steps. Under CPython's GIL those steps could not interleave with another
thread, so ` + "`counter += 1`" + ` behaved as if it were atomic. Without the GIL,
and on unagi, two threads can read the same value, each add one, and each store,
so one update is lost.

    counter = 0

    def worker():
        global counter
        for _ in range(100_000):
            counter += 1          # racy: read, add, store

The value stays a well-formed int and nothing crashes, but the final total is
smaller than it should be. Guard the update with a lock so the three steps run as
one:

    lock = threading.Lock()

    def worker():
        global counter
        for _ in range(100_000):
            with lock:
                counter += 1

The faster fix needs no lock at all: give each worker a private counter and
combine the results after join, so the threads never share a mutable cell.

    def worker(out, i):
        total = 0
        for _ in range(100_000):
            total += 1
        out[i] = total            # one writer per slot, summed after join
`
