package vet

import "testing"

// suppress parses source, analyzes it, and returns the kept findings and the
// suppressed count in one step.
func suppress(t *testing.T, src string) ([]Finding, int) {
	t.Helper()
	return Suppress([]byte(src), analyze(t, src))
}

func TestSuppressWaivesTaggedLine(t *testing.T) {
	const src = `import threading

counter = 0

def worker():
    global counter
    counter += 1  # unagi: ok UNA-THR-001

threading.Thread(target=worker).start()
`
	kept, n := suppress(t, src)
	if len(kept) != 0 || n != 1 {
		t.Fatalf("want 0 kept and 1 suppressed, got %d kept %d suppressed", len(kept), n)
	}
}

func TestSuppressBareWaivesAnyCode(t *testing.T) {
	const src = `import threading

counter = 0

def worker():
    global counter
    counter += 1  # unagi: ok

threading.Thread(target=worker).start()
`
	if kept, n := suppress(t, src); len(kept) != 0 || n != 1 {
		t.Fatalf("bare ok should waive, got %d kept %d suppressed", len(kept), n)
	}
}

func TestSuppressWrongCodeStillFires(t *testing.T) {
	const src = `import threading

counter = 0

def worker():
    global counter
    counter += 1  # unagi: ok UNA-THR-999

threading.Thread(target=worker).start()
`
	kept, n := suppress(t, src)
	if len(kept) != 1 || n != 0 {
		t.Fatalf("a mismatched code should not waive, got %d kept %d suppressed", len(kept), n)
	}
}

func TestSuppressLeavesUntaggedLines(t *testing.T) {
	const src = `import threading

counter = 0
hits = {}

def worker(name):
    global counter
    counter += 1  # unagi: ok UNA-THR-001
    hits[name] = hits.get(name, 0) + 1

threading.Thread(target=worker, args=("t0",)).start()
`
	kept, n := suppress(t, src)
	if len(kept) != 1 || n != 1 {
		t.Fatalf("want the hits finding kept and counter suppressed, got %d kept %d suppressed", len(kept), n)
	}
	if kept[0].Pos.Line != 9 {
		t.Fatalf("kept finding should be the hits line, got line %d", kept[0].Pos.Line)
	}
}

func TestSuppressIgnoresHashInString(t *testing.T) {
	// The `#` sits inside a string, so it is not a suppression comment and the
	// finding stands.
	const src = `import threading

counter = 0

def worker():
    global counter
    tag = "# unagi: ok UNA-THR-001"
    counter += 1

threading.Thread(target=worker).start()
`
	if kept, n := suppress(t, src); len(kept) != 1 || n != 0 {
		t.Fatalf("a hash inside a string should not waive, got %d kept %d suppressed", len(kept), n)
	}
}
