package vet

import (
	"strings"
	"testing"
)

func TestOpenChainFires(t *testing.T) {
	const src = `import threading

def load(path):
    return open(path).read()

threading.Thread(target=load, args=("f",)).start()
`
	fs := analyze(t, src)
	if got := codes(fs); len(got) != 1 || got[0] != "UNA-THR-006" {
		t.Fatalf("want one UNA-THR-006, got %v", got)
	}
	if fs[0].Pos.Line != 4 || !strings.Contains(fs[0].Msg, "open()") {
		t.Errorf("finding: line %d msg %q", fs[0].Pos.Line, fs[0].Msg)
	}
}

func TestOpenForIterFires(t *testing.T) {
	const src = `import threading

def load(path):
    for line in open(path):
        handle(line)

threading.Thread(target=load, args=("f",)).start()
`
	if got := codes(analyze(t, src)); len(got) != 1 || got[0] != "UNA-THR-006" {
		t.Fatalf("open() as a loop iterable should fire, got %v", got)
	}
}

func TestWithOpenIsSilent(t *testing.T) {
	const src = `import threading

def load(path):
    with open(path) as f:
        return f.read()

threading.Thread(target=load, args=("f",)).start()
`
	if fs := analyze(t, src); len(fs) != 0 {
		t.Fatalf("with open should be silent, got %v", codes(fs))
	}
}

func TestBoundFileIsSilent(t *testing.T) {
	// The file is bound to a name, so its lifetime is under the author's control
	// rather than a discarded temporary; this check does not chase it.
	const src = `import threading

def load(path):
    f = open(path)
    data = f.read()
    f.close()
    return data

threading.Thread(target=load, args=("f",)).start()
`
	if fs := analyze(t, src); len(fs) != 0 {
		t.Fatalf("a bound file should be silent, got %v", codes(fs))
	}
}

func TestMethodOpenIsSilent(t *testing.T) {
	// db.open() is a method named open, not the builtin, so its result is not a
	// file relying on prompt finalization.
	const src = `import threading

def load(db):
    return db.open().fetch()

threading.Thread(target=load, args=(None,)).start()
`
	if fs := analyze(t, src); len(fs) != 0 {
		t.Fatalf("a method named open should be silent, got %v", codes(fs))
	}
}

func TestOpenSingleThreadedIsSilent(t *testing.T) {
	const src = `def load(path):
    return open(path).read()

load("f")
`
	if fs := analyze(t, src); len(fs) != 0 {
		t.Fatalf("single-threaded program should be silent, got %v", codes(fs))
	}
}
