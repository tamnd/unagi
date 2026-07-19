package vet

import (
	"strings"
	"testing"
)

func TestManualAcquireFires(t *testing.T) {
	const src = `import threading

lock = threading.Lock()

def work():
    lock.acquire()
    do_work()
    lock.release()

threading.Thread(target=work).start()
`
	fs := analyze(t, src)
	if got := codes(fs); len(got) != 1 || got[0] != "UNA-THR-005" {
		t.Fatalf("want one UNA-THR-005, got %v", got)
	}
	if fs[0].Pos.Line != 6 || !strings.Contains(fs[0].Msg, "'lock'") {
		t.Errorf("finding: line %d msg %q", fs[0].Pos.Line, fs[0].Msg)
	}
}

func TestAcquireWithNoReleaseFires(t *testing.T) {
	const src = `import threading

lock = threading.Lock()

def work():
    lock.acquire()
    do_work()

threading.Thread(target=work).start()
`
	if got := codes(analyze(t, src)); len(got) != 1 || got[0] != "UNA-THR-005" {
		t.Fatalf("a leaked lock should fire, got %v", got)
	}
}

func TestTryFinallyReleaseIsSilent(t *testing.T) {
	const src = `import threading

lock = threading.Lock()

def work():
    lock.acquire()
    try:
        do_work()
    finally:
        lock.release()

threading.Thread(target=work).start()
`
	if fs := analyze(t, src); len(fs) != 0 {
		t.Fatalf("try/finally release should be silent, got %v", codes(fs))
	}
}

func TestWithLockIsSilent(t *testing.T) {
	const src = `import threading

lock = threading.Lock()

def work():
    with lock:
        do_work()

threading.Thread(target=work).start()
`
	if fs := analyze(t, src); len(fs) != 0 {
		t.Fatalf("with lock should be silent, got %v", codes(fs))
	}
}

func TestTryLockIsSilent(t *testing.T) {
	// acquire used for its return value is a deliberate try-lock, not the
	// unconditional acquire this check targets.
	const src = `import threading

lock = threading.Lock()

def work():
    if lock.acquire(blocking=False):
        try:
            do_work()
        finally:
            lock.release()

threading.Thread(target=work).start()
`
	if fs := analyze(t, src); len(fs) != 0 {
		t.Fatalf("a try-lock should be silent, got %v", codes(fs))
	}
}

func TestNonLockAcquireIsSilent(t *testing.T) {
	// conn is not a lock, so its acquire is some other method and not our concern.
	const src = `import threading

conn = open_pool()

def work():
    conn.acquire()
    conn.release()

threading.Thread(target=work).start()
`
	if fs := analyze(t, src); len(fs) != 0 {
		t.Fatalf("a non-lock acquire should be silent, got %v", codes(fs))
	}
}
