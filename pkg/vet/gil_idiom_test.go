package vet

import (
	"strings"
	"testing"
)

func TestSpinWaitFires(t *testing.T) {
	const src = `import threading

done = False

def worker():
    global done
    run()
    done = True

threading.Thread(target=worker).start()
while not done:
    pass
`
	fs := analyze(t, src)
	if got := codes(fs); len(got) != 1 || got[0] != "UNA-THR-004" {
		t.Fatalf("want one UNA-THR-004, got %v", got)
	}
	if fs[0].Pos.Line != 11 || !strings.Contains(fs[0].Msg, "'done'") {
		t.Errorf("finding: line %d msg %q", fs[0].Pos.Line, fs[0].Msg)
	}
}

func TestSleepPollFires(t *testing.T) {
	const src = `import threading
import time

ready = False

def worker():
    global ready
    ready = True

threading.Thread(target=worker).start()
while not ready:
    time.sleep(0.01)
`
	if got := codes(analyze(t, src)); len(got) != 1 || got[0] != "UNA-THR-004" {
		t.Fatalf("a sleep poll should fire, got %v", got)
	}
}

func TestWorkingLoopIsSilent(t *testing.T) {
	// The loop does real work each iteration, so it is not a spin-wait.
	const src = `import threading

done = False

def worker():
    global done
    done = True

threading.Thread(target=worker).start()
while not done:
    step()
`
	if fs := analyze(t, src); len(fs) != 0 {
		t.Fatalf("a working loop should be silent, got %v", codes(fs))
	}
}

func TestSpinOnConstantIsSilent(t *testing.T) {
	// `while True` polls no shared flag, so it is a normal event loop, not the
	// GIL-relict wait this check targets.
	const src = `import threading

def worker():
    pass

threading.Thread(target=worker).start()
while True:
    pass
`
	if fs := analyze(t, src); len(fs) != 0 {
		t.Fatalf("while True should be silent, got %v", codes(fs))
	}
}

func TestSpinOnUnwrittenFlagIsSilent(t *testing.T) {
	// flag is never assigned in any function, so no thread flips it and the
	// loop is not the cross-thread signal this check targets.
	const src = `import threading

flag = False

def worker():
    compute()

threading.Thread(target=worker).start()
while not flag:
    pass
`
	if fs := analyze(t, src); len(fs) != 0 {
		t.Fatalf("spin on an unwritten flag should be silent, got %v", codes(fs))
	}
}
