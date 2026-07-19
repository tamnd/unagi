package vet

import (
	"strings"
	"testing"
)

func TestTypedGlobalRebindFires(t *testing.T) {
	const src = `import threading

counter: int = 0

def reset():
    global counter
    counter = fresh()

threading.Thread(target=reset).start()
`
	fs := analyze(t, src)
	if got := codes(fs); len(got) != 1 || got[0] != "UNA-THR-008" {
		t.Fatalf("want one UNA-THR-008, got %v", got)
	}
	if fs[0].Pos.Line != 7 || !strings.Contains(fs[0].Msg, "'counter'") {
		t.Errorf("finding: line %d msg %q", fs[0].Pos.Line, fs[0].Msg)
	}
}

func TestUnannotatedGlobalRebindIsSilent(t *testing.T) {
	// No annotation means no typed shadow, so the cross-tier surprise does not
	// arise; this is a plain boxed global.
	const src = `import threading

counter = 0

def reset():
    global counter
    counter = fresh()

threading.Thread(target=reset).start()
`
	if got := codes(analyze(t, src)); len(got) != 0 {
		t.Fatalf("an unannotated global rebind should not fire UNA-THR-008, got %v", got)
	}
}

func TestTypedGlobalReadOnlyIsSilent(t *testing.T) {
	// The thread only reads the annotated global, so there is no rebind to make
	// a shadow go stale.
	const src = `import threading

limit: int = 100

def check(x):
    return x < limit

threading.Thread(target=check, args=(1,)).start()
`
	if fs := analyze(t, src); len(fs) != 0 {
		t.Fatalf("read-only typed global should be silent, got %v", codes(fs))
	}
}

func TestTypedGlobalRebindNoThreadIsSilent(t *testing.T) {
	const src = `counter: int = 0

def reset():
    global counter
    counter = fresh()

reset()
`
	if fs := analyze(t, src); len(fs) != 0 {
		t.Fatalf("single-threaded rebind should be silent, got %v", codes(fs))
	}
}

func TestTypedGlobalRMWFiresBothCodes(t *testing.T) {
	// A read-modify-write of an annotated global is both a lost-update race and
	// a cross-tier shadow hazard, so both codes are reported.
	const src = `import threading

counter: int = 0

def bump():
    global counter
    counter = counter + 1

threading.Thread(target=bump).start()
`
	got := codes(analyze(t, src))
	if len(got) != 2 || got[0] != "UNA-THR-001" || got[1] != "UNA-THR-008" {
		t.Fatalf("want UNA-THR-001 then UNA-THR-008 on the same line, got %v", got)
	}
}
