package vet

import (
	"strings"
	"testing"

	"github.com/tamnd/unagi/pkg/frontend"
)

// analyze parses source and returns the findings, failing the test on a parse
// error so a malformed case is never silently read as clean.
func analyze(t *testing.T, src string) []Finding {
	t.Helper()
	mod, err := frontend.Parse([]byte(src), "test.py")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return Analyze(mod)
}

// codes lists the finding codes in order, for compact assertions.
func codes(fs []Finding) []string {
	out := make([]string, len(fs))
	for i, f := range fs {
		out[i] = f.Code
	}
	return out
}

func TestSharedCounterFiresTwice(t *testing.T) {
	const src = `import threading

counter = 0
hits = {}

def worker(name):
    global counter
    for _ in range(100000):
        counter += 1
        hits[name] = hits.get(name, 0) + 1

threads = [threading.Thread(target=worker, args=(f"t{i}",)) for i in range(4)]
for t in threads:
    t.start()
for t in threads:
    t.join()
print(counter)
`
	fs := analyze(t, src)
	if got := codes(fs); len(got) != 2 || got[0] != "UNA-THR-001" || got[1] != "UNA-THR-001" {
		t.Fatalf("want two UNA-THR-001 findings, got %v", got)
	}
	if fs[0].Pos.Line != 9 || !strings.Contains(fs[0].Msg, "'counter'") {
		t.Errorf("counter finding: line %d msg %q", fs[0].Pos.Line, fs[0].Msg)
	}
	if fs[1].Pos.Line != 10 || !strings.Contains(fs[1].Msg, "'hits'") {
		t.Errorf("hits finding: line %d msg %q", fs[1].Pos.Line, fs[1].Msg)
	}
}

func TestLockedCounterIsSilent(t *testing.T) {
	const src = `import threading

counter = 0
hits = {}
lock = threading.Lock()

def worker(name):
    global counter
    for _ in range(100000):
        with lock:
            counter += 1
            hits[name] = hits.get(name, 0) + 1

threading.Thread(target=worker, args=("t0",)).start()
`
	if fs := analyze(t, src); len(fs) != 0 {
		t.Fatalf("locked program should be silent, got %v", codes(fs))
	}
}

func TestSingleThreadedIsSilent(t *testing.T) {
	const src = `counter = 0
hits = {}

def worker(name):
    global counter
    counter += 1
    hits[name] = hits.get(name, 0) + 1

worker("t0")
print(counter)
`
	if fs := analyze(t, src); len(fs) != 0 {
		t.Fatalf("single-threaded program should be silent, got %v", codes(fs))
	}
}

func TestLocalNameIsNotShared(t *testing.T) {
	// counter here is a plain local (no `global`), so `counter += 1` binds a
	// local and is not a shared read-modify-write.
	const src = `import threading

def worker():
    counter = 0
    counter += 1
    return counter

threading.Thread(target=worker).start()
`
	if fs := analyze(t, src); len(fs) != 0 {
		t.Fatalf("local counter should not fire, got %v", codes(fs))
	}
}

func TestAttributeRMWFires(t *testing.T) {
	const src = `import threading

state = object()

def worker():
    state.n = state.n + 1

threading.Thread(target=worker).start()
`
	fs := analyze(t, src)
	if got := codes(fs); len(got) != 1 || got[0] != "UNA-THR-001" {
		t.Fatalf("want one UNA-THR-001, got %v", got)
	}
	if !strings.Contains(fs[0].Msg, "'state.n'") {
		t.Errorf("attribute finding msg %q", fs[0].Msg)
	}
}

func TestExecutorGatesTheCheck(t *testing.T) {
	const src = `from concurrent.futures import ThreadPoolExecutor

total = {}

def worker(k):
    total[k] = total.get(k, 0) + 1

with ThreadPoolExecutor() as ex:
    ex.submit(worker, "a")
`
	fs := analyze(t, src)
	if got := codes(fs); len(got) != 1 || got[0] != "UNA-THR-001" {
		t.Fatalf("executor should gate the check on, got %v", got)
	}
}

func TestShadowedGlobalDoesNotFire(t *testing.T) {
	// hits is rebound as a local inside worker, so the subscript touches the
	// local, not the module global.
	const src = `import threading

hits = {}

def worker(name):
    hits = {}
    hits[name] = hits.get(name, 0) + 1
    return hits

threading.Thread(target=worker, args=("t0",)).start()
`
	if fs := analyze(t, src); len(fs) != 0 {
		t.Fatalf("shadowed local should not fire, got %v", codes(fs))
	}
}
