package vet

import (
	"strings"
	"testing"
)

func TestBlockingSleepInCoroutineFires(t *testing.T) {
	const src = `import asyncio
import time

async def handle():
    time.sleep(1)
`
	fs := analyze(t, src)
	if got := codes(fs); len(got) != 1 || got[0] != "UNA-AIO-001" {
		t.Fatalf("want one UNA-AIO-001, got %v", got)
	}
	if fs[0].Pos.Line != 5 || !strings.Contains(fs[0].Msg, "time.sleep") || !strings.Contains(fs[0].Msg, "'handle'") {
		t.Errorf("finding: line %d msg %q", fs[0].Pos.Line, fs[0].Msg)
	}
}

func TestBlockingRequestsInCoroutineFires(t *testing.T) {
	const src = `import requests

async def fetch(url):
    return requests.get(url)
`
	fs := analyze(t, src)
	if got := codes(fs); len(got) != 1 || got[0] != "UNA-AIO-001" {
		t.Fatalf("want one UNA-AIO-001, got %v", got)
	}
	if !strings.Contains(fs[0].Msg, "requests.get") {
		t.Errorf("msg %q", fs[0].Msg)
	}
}

func TestBlockingOpenInCoroutineFires(t *testing.T) {
	const src = `async def load(path):
    data = open(path).read()
    return data
`
	if got := codes(analyze(t, src)); len(got) != 1 || got[0] != "UNA-AIO-001" {
		t.Fatalf("bare open in a coroutine should fire UNA-AIO-001, got %v", got)
	}
}

func TestAwaitedSleepIsSilent(t *testing.T) {
	const src = `import asyncio

async def handle():
    await asyncio.sleep(1)
`
	if fs := analyze(t, src); len(fs) != 0 {
		t.Fatalf("awaiting asyncio.sleep should be silent, got %v", codes(fs))
	}
}

func TestBlockingCallInSyncFunctionIsSilent(t *testing.T) {
	// The same call in a plain function is fine; only a coroutine stalls a loop.
	const src = `import time

def handle():
    time.sleep(1)
`
	if fs := analyze(t, src); len(fs) != 0 {
		t.Fatalf("blocking call in a sync function should be silent, got %v", codes(fs))
	}
}

func TestOffloadedBlockingCallIsSilent(t *testing.T) {
	// Passing the function as a value to run_in_executor is an offload, not a
	// call in the coroutine, so it stays quiet.
	const src = `import time

async def handle(loop):
    await loop.run_in_executor(None, time.sleep, 1)
`
	if fs := analyze(t, src); len(fs) != 0 {
		t.Fatalf("run_in_executor offload should be silent, got %v", codes(fs))
	}
}

func TestBlockingCallInNestedSyncDefIsSilent(t *testing.T) {
	// A plain def nested in a coroutine runs in its own context, not on the
	// loop, so a blocking call there is not the coroutine's hazard.
	const src = `import time

async def handle():
    def worker():
        time.sleep(1)
    return worker
`
	if fs := analyze(t, src); len(fs) != 0 {
		t.Fatalf("blocking call in a nested sync def should be silent, got %v", codes(fs))
	}
}

func TestBlockingCallInNestedCoroutineFires(t *testing.T) {
	// A nested async def is itself a coroutine, so a blocking call inside it is
	// flagged against the nested function.
	const src = `import time

async def outer():
    async def inner():
        time.sleep(1)
    return inner
`
	fs := analyze(t, src)
	if got := codes(fs); len(got) != 1 || got[0] != "UNA-AIO-001" {
		t.Fatalf("want one UNA-AIO-001 for the nested coroutine, got %v", got)
	}
	if !strings.Contains(fs[0].Msg, "'inner'") {
		t.Errorf("want the nested function named, msg %q", fs[0].Msg)
	}
}
