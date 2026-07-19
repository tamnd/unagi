package vet

import (
	"strings"
	"testing"
)

func TestCheckThenActDictFires(t *testing.T) {
	const src = `import threading

cache = {}

def get(key):
    if key not in cache:
        cache[key] = build(key)
    return cache[key]

threading.Thread(target=get, args=("k",)).start()
`
	fs := analyze(t, src)
	if got := codes(fs); len(got) != 1 || got[0] != "UNA-THR-002" {
		t.Fatalf("want one UNA-THR-002, got %v", got)
	}
	if fs[0].Pos.Line != 6 || !strings.Contains(fs[0].Msg, "'cache'") {
		t.Errorf("finding: line %d msg %q", fs[0].Pos.Line, fs[0].Msg)
	}
}

func TestCheckThenActLazyInitFires(t *testing.T) {
	const src = `import threading

conn = None

def connect():
    global conn
    if conn is None:
        conn = open_socket()
    return conn

threading.Thread(target=connect).start()
`
	fs := analyze(t, src)
	if got := codes(fs); len(got) != 1 || got[0] != "UNA-THR-002" {
		t.Fatalf("want one UNA-THR-002, got %v", got)
	}
	if !strings.Contains(fs[0].Msg, "'conn'") {
		t.Errorf("finding msg %q", fs[0].Msg)
	}
}

func TestCheckThenActMethodMutationFires(t *testing.T) {
	const src = `import threading

seen = set()

def note(x):
    if x not in seen:
        seen.add(x)

threading.Thread(target=note, args=(1,)).start()
`
	if got := codes(analyze(t, src)); len(got) != 1 || got[0] != "UNA-THR-002" {
		t.Fatalf("want one UNA-THR-002, got %v", got)
	}
}

func TestCheckThenActUnderLockIsSilent(t *testing.T) {
	const src = `import threading

cache = {}
lock = threading.Lock()

def get(key):
    with lock:
        if key not in cache:
            cache[key] = build(key)
    return cache[key]

threading.Thread(target=get, args=("k",)).start()
`
	if fs := analyze(t, src); len(fs) != 0 {
		t.Fatalf("check-then-act under a lock should be silent, got %v", codes(fs))
	}
}

func TestCheckThenActSingleThreadedIsSilent(t *testing.T) {
	const src = `cache = {}

def get(key):
    if key not in cache:
        cache[key] = build(key)
    return cache[key]

get("k")
`
	if fs := analyze(t, src); len(fs) != 0 {
		t.Fatalf("single-threaded program should be silent, got %v", codes(fs))
	}
}

func TestCheckThenActLocalIsNotShared(t *testing.T) {
	// cache is a fresh local, so its check-then-act is not shared across threads.
	const src = `import threading

def get(key):
    cache = {}
    if key not in cache:
        cache[key] = build(key)
    return cache[key]

threading.Thread(target=get, args=("k",)).start()
`
	if fs := analyze(t, src); len(fs) != 0 {
		t.Fatalf("local cache should not fire, got %v", codes(fs))
	}
}

func TestCheckThenActUnrelatedMutationIsSilent(t *testing.T) {
	// The condition checks flag but the body mutates a different global, so
	// there is no check-then-act window on one object.
	const src = `import threading

flag = True
log = []

def run(x):
    if flag:
        log.append(x)

threading.Thread(target=run, args=(1,)).start()
`
	if got := codes(analyze(t, src)); len(got) != 0 {
		t.Fatalf("mismatched check and act should not fire UNA-THR-002, got %v", got)
	}
}
