package vet

import (
	"strings"
	"testing"
)

func TestProcessLambdaTargetFires(t *testing.T) {
	const src = `import multiprocessing

multiprocessing.Process(target=lambda: work()).start()
`
	fs := analyze(t, src)
	if got := codes(fs); len(got) != 1 || got[0] != "UNA-MP-002" {
		t.Fatalf("want one UNA-MP-002, got %v", got)
	}
	if !strings.Contains(fs[0].Msg, "lambda") {
		t.Errorf("msg %q", fs[0].Msg)
	}
}

func TestPoolMapLambdaFires(t *testing.T) {
	const src = `import multiprocessing

with multiprocessing.Pool() as pool:
    pool.map(lambda x: x * 2, items)
`
	if got := codes(analyze(t, src)); len(got) != 1 || got[0] != "UNA-MP-002" {
		t.Fatalf("a lambda passed to pool.map should fire, got %v", got)
	}
}

func TestPoolMapClosureFires(t *testing.T) {
	const src = `import multiprocessing

def outer(config):
    def job(item):
        return item + config
    with multiprocessing.Pool() as pool:
        return pool.map(job, items)
`
	fs := analyze(t, src)
	if got := codes(fs); len(got) != 1 || got[0] != "UNA-MP-002" {
		t.Fatalf("a closure passed to pool.map should fire, got %v", got)
	}
	if !strings.Contains(fs[0].Msg, "'job'") {
		t.Errorf("want the closure named, msg %q", fs[0].Msg)
	}
}

func TestModuleLevelTargetIsSilent(t *testing.T) {
	const src = `import multiprocessing

def job(item):
    return item * 2

with multiprocessing.Pool() as pool:
    pool.map(job, items)

multiprocessing.Process(target=job).start()
`
	if fs := analyze(t, src); len(fs) != 0 {
		t.Fatalf("a module-level target should be silent, got %v", codes(fs))
	}
}

func TestBoundMethodTargetIsSilent(t *testing.T) {
	// A bound method pickles fine when its object does, so it is not flagged.
	const src = `import multiprocessing

multiprocessing.Process(target=worker.run).start()
`
	if fs := analyze(t, src); len(fs) != 0 {
		t.Fatalf("a bound method should be silent, got %v", codes(fs))
	}
}

func TestShadowedClosureNameIsSilent(t *testing.T) {
	// job exists at module level too, so a use could resolve to the picklable
	// one; the check does not fire on the ambiguous name.
	const src = `import multiprocessing

def job(item):
    return item

def outer():
    def job(item):
        return item + 1
    return job

multiprocessing.Process(target=job).start()
`
	if fs := analyze(t, src); len(fs) != 0 {
		t.Fatalf("a name also defined at module level should be silent, got %v", codes(fs))
	}
}
