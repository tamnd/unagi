package vet

import (
	"strings"
	"testing"
)

func TestSetStartMethodForkFires(t *testing.T) {
	const src = `import multiprocessing

multiprocessing.set_start_method("fork")
`
	fs := analyze(t, src)
	if got := codes(fs); len(got) != 1 || got[0] != "UNA-MP-001" {
		t.Fatalf("want one UNA-MP-001, got %v", got)
	}
	if fs[0].Pos.Line != 3 || !strings.Contains(fs[0].Msg, "'fork'") {
		t.Errorf("finding: line %d msg %q", fs[0].Pos.Line, fs[0].Msg)
	}
}

func TestGetContextForkFires(t *testing.T) {
	const src = `import multiprocessing

ctx = multiprocessing.get_context("fork")
`
	if got := codes(analyze(t, src)); len(got) != 1 || got[0] != "UNA-MP-001" {
		t.Fatalf("get_context fork should fire, got %v", got)
	}
}

func TestForkserverFires(t *testing.T) {
	const src = `import multiprocessing

multiprocessing.set_start_method(method="forkserver")
`
	fs := analyze(t, src)
	if got := codes(fs); len(got) != 1 || got[0] != "UNA-MP-001" {
		t.Fatalf("forkserver should fire, got %v", got)
	}
	if !strings.Contains(fs[0].Msg, "'forkserver'") {
		t.Errorf("msg %q", fs[0].Msg)
	}
}

func TestSpawnIsSilent(t *testing.T) {
	const src = `import multiprocessing

multiprocessing.set_start_method("spawn")
ctx = multiprocessing.get_context("spawn")
`
	if fs := analyze(t, src); len(fs) != 0 {
		t.Fatalf("spawn should be silent, got %v", codes(fs))
	}
}

func TestGetContextNoArgIsSilent(t *testing.T) {
	const src = `import multiprocessing

ctx = multiprocessing.get_context()
`
	if fs := analyze(t, src); len(fs) != 0 {
		t.Fatalf("get_context with no method should be silent, got %v", codes(fs))
	}
}
