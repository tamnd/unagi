package vet

import (
	"strings"
	"testing"
)

func TestSharedIterForFires(t *testing.T) {
	const src = `import threading

items = []

def produce(x):
    items.append(x)

def consume():
    for x in items:
        handle(x)

threading.Thread(target=produce, args=(1,)).start()
threading.Thread(target=consume).start()
`
	fs := analyze(t, src)
	if got := codes(fs); len(got) != 1 || got[0] != "UNA-THR-003" {
		t.Fatalf("want one UNA-THR-003, got %v", got)
	}
	if fs[0].Pos.Line != 9 || !strings.Contains(fs[0].Msg, "'items'") {
		t.Errorf("finding: line %d msg %q", fs[0].Pos.Line, fs[0].Msg)
	}
}

func TestSharedIterDictViewFires(t *testing.T) {
	const src = `import threading

table = {}

def add(k, v):
    table[k] = v

def scan():
    for k in table.keys():
        use(k)

threading.Thread(target=add, args=("a", 1)).start()
threading.Thread(target=scan).start()
`
	if got := codes(analyze(t, src)); len(got) != 1 || got[0] != "UNA-THR-003" {
		t.Fatalf("dict view iteration should fire, got %v", got)
	}
}

func TestSharedIterComprehensionFires(t *testing.T) {
	const src = `import threading

items = []

def produce(x):
    items.append(x)

def snapshot():
    return [x * 2 for x in items]

threading.Thread(target=produce, args=(1,)).start()
threading.Thread(target=snapshot).start()
`
	if got := codes(analyze(t, src)); len(got) != 1 || got[0] != "UNA-THR-003" {
		t.Fatalf("comprehension over a mutated global should fire, got %v", got)
	}
}

func TestSharedIterSnapshotIsSilent(t *testing.T) {
	const src = `import threading

items = []

def produce(x):
    items.append(x)

def consume():
    for x in list(items):
        handle(x)

threading.Thread(target=produce, args=(1,)).start()
threading.Thread(target=consume).start()
`
	if fs := analyze(t, src); len(fs) != 0 {
		t.Fatalf("iterating a snapshot should be silent, got %v", codes(fs))
	}
}

func TestSharedIterUnderLockIsSilent(t *testing.T) {
	const src = `import threading

items = []
lock = threading.Lock()

def produce(x):
    with lock:
        items.append(x)

def consume():
    with lock:
        for x in items:
            handle(x)

threading.Thread(target=produce, args=(1,)).start()
threading.Thread(target=consume).start()
`
	if fs := analyze(t, src); len(fs) != 0 {
		t.Fatalf("iterating under a lock should be silent, got %v", codes(fs))
	}
}

func TestSharedIterReadOnlyGlobalIsSilent(t *testing.T) {
	// config is built once at module scope and never mutated in a function, so
	// iterating it across threads is safe.
	const src = `import threading

config = ["a", "b", "c"]

def consume():
    for x in config:
        handle(x)

threading.Thread(target=consume).start()
`
	if fs := analyze(t, src); len(fs) != 0 {
		t.Fatalf("read-only global iteration should be silent, got %v", codes(fs))
	}
}

func TestSharedIterLocalIsNotShared(t *testing.T) {
	const src = `import threading

def consume():
    items = []
    items.append(1)
    for x in items:
        handle(x)

threading.Thread(target=consume).start()
`
	if fs := analyze(t, src); len(fs) != 0 {
		t.Fatalf("local container iteration should be silent, got %v", codes(fs))
	}
}
