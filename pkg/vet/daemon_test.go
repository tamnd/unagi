package vet

import (
	"strings"
	"testing"
)

func TestDaemonWriterFires(t *testing.T) {
	const src = `import threading

def log_forever():
    with open("out.log", "a") as f:
        while True:
            f.write(next_line())

threading.Thread(target=log_forever, daemon=True).start()
`
	fs := analyze(t, src)
	if got := codes(fs); len(got) != 1 || got[0] != "UNA-THR-007" {
		t.Fatalf("want one UNA-THR-007, got %v", got)
	}
	if fs[0].Pos.Line != 8 || !strings.Contains(fs[0].Msg, "'log_forever'") {
		t.Errorf("finding: line %d msg %q", fs[0].Pos.Line, fs[0].Msg)
	}
}

func TestNonDaemonWriterIsSilent(t *testing.T) {
	const src = `import threading

def log_forever():
    with open("out.log", "a") as f:
        f.write("x")

t = threading.Thread(target=log_forever)
t.start()
t.join()
`
	if fs := analyze(t, src); len(fs) != 0 {
		t.Fatalf("a joined non-daemon writer should be silent, got %v", codes(fs))
	}
}

func TestDaemonComputeIsSilent(t *testing.T) {
	// The daemon only computes, holding no resource to lose at exit.
	const src = `import threading

def crunch():
    total = 0
    for i in range(1000):
        total += i
    return total

threading.Thread(target=crunch, daemon=True).start()
`
	if fs := analyze(t, src); len(fs) != 0 {
		t.Fatalf("a compute-only daemon should be silent, got %v", codes(fs))
	}
}

func TestDaemonFalseIsSilent(t *testing.T) {
	const src = `import threading

def log_forever():
    with open("out.log", "a") as f:
        f.write("x")

threading.Thread(target=log_forever, daemon=False).start()
`
	if fs := analyze(t, src); len(fs) != 0 {
		t.Fatalf("daemon=False should be silent, got %v", codes(fs))
	}
}
