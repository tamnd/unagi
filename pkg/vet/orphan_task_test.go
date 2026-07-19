package vet

import (
	"strings"
	"testing"
)

func TestOrphanCreateTaskFires(t *testing.T) {
	const src = `import asyncio

async def main():
    asyncio.create_task(worker())
    await serve()
`
	fs := analyze(t, src)
	if got := codes(fs); len(got) != 1 || got[0] != "UNA-AIO-002" {
		t.Fatalf("want one UNA-AIO-002, got %v", got)
	}
	if fs[0].Pos.Line != 4 || !strings.Contains(fs[0].Msg, "create_task") {
		t.Errorf("finding: line %d msg %q", fs[0].Pos.Line, fs[0].Msg)
	}
}

func TestBareImportedCreateTaskFires(t *testing.T) {
	const src = `from asyncio import create_task

async def main():
    create_task(worker())
`
	if got := codes(analyze(t, src)); len(got) != 1 || got[0] != "UNA-AIO-002" {
		t.Fatalf("a bare imported create_task should fire, got %v", got)
	}
}

func TestStoredTaskIsSilent(t *testing.T) {
	const src = `import asyncio

async def main():
    task = asyncio.create_task(worker())
    await task
`
	if fs := analyze(t, src); len(fs) != 0 {
		t.Fatalf("a stored task should be silent, got %v", codes(fs))
	}
}

func TestAppendedTaskIsSilent(t *testing.T) {
	const src = `import asyncio

async def main():
    tasks = []
    tasks.append(asyncio.create_task(worker()))
`
	if fs := analyze(t, src); len(fs) != 0 {
		t.Fatalf("a task kept in a list should be silent, got %v", codes(fs))
	}
}

func TestAwaitedCreateTaskIsSilent(t *testing.T) {
	const src = `import asyncio

async def main():
    await asyncio.create_task(worker())
`
	if fs := analyze(t, src); len(fs) != 0 {
		t.Fatalf("an awaited create_task should be silent, got %v", codes(fs))
	}
}

func TestTaskGroupCreateTaskIsSilent(t *testing.T) {
	// A TaskGroup owns and awaits its children, so tg.create_task is not
	// orphaned even as a bare statement.
	const src = `import asyncio

async def main():
    async with asyncio.TaskGroup() as tg:
        tg.create_task(worker())
`
	if fs := analyze(t, src); len(fs) != 0 {
		t.Fatalf("TaskGroup.create_task should be silent, got %v", codes(fs))
	}
}
