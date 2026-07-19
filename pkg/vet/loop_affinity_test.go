package vet

import (
	"strings"
	"testing"
)

func TestLoopCallSoonFromThreadFires(t *testing.T) {
	const src = `import threading

def worker(loop, fut):
    result = compute()
    loop.call_soon(fut.set_result, result)

threading.Thread(target=worker, args=(loop, fut)).start()
`
	fs := analyze(t, src)
	if got := codes(fs); len(got) != 1 || got[0] != "UNA-AIO-003" {
		t.Fatalf("want one UNA-AIO-003, got %v", got)
	}
	if fs[0].Pos.Line != 5 || !strings.Contains(fs[0].Msg, "call_soon") || !strings.Contains(fs[0].Msg, "'worker'") {
		t.Errorf("finding: line %d msg %q", fs[0].Pos.Line, fs[0].Msg)
	}
}

func TestFutureSetResultFromThreadFires(t *testing.T) {
	const src = `import threading

def worker(fut):
    fut.set_result(compute())

threading.Thread(target=worker, args=(fut,)).start()
`
	if got := codes(analyze(t, src)); len(got) != 1 || got[0] != "UNA-AIO-003" {
		t.Fatalf("a set_result from a thread should fire, got %v", got)
	}
}

func TestThreadsafeCallIsSilent(t *testing.T) {
	const src = `import threading

def worker(loop, fut):
    loop.call_soon_threadsafe(fut.set_result, compute())

threading.Thread(target=worker, args=(loop, fut)).start()
`
	if fs := analyze(t, src); len(fs) != 0 {
		t.Fatalf("call_soon_threadsafe should be silent, got %v", codes(fs))
	}
}

func TestLoopCallSoonOutsideThreadIsSilent(t *testing.T) {
	// call_soon on the loop's own thread is the normal, correct use.
	const src = `import asyncio

async def main():
    loop = asyncio.get_running_loop()
    loop.call_soon(do_thing)
`
	if fs := analyze(t, src); len(fs) != 0 {
		t.Fatalf("call_soon outside a thread target should be silent, got %v", codes(fs))
	}
}

func TestRunInExecutorCallbackFires(t *testing.T) {
	const src = `import threading

def worker(fut):
    fut.set_result(compute())

def start(loop, fut):
    loop.run_in_executor(None, worker, fut)

threading.Thread(target=start, args=(loop, fut)).start()
`
	// worker runs off the loop thread via run_in_executor, so its set_result
	// fires; start only schedules it, so start itself is clean.
	if got := codes(analyze(t, src)); len(got) != 1 || got[0] != "UNA-AIO-003" {
		t.Fatalf("a run_in_executor callback touching a future should fire, got %v", got)
	}
}
