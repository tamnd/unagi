package vet

// explanations holds the long-form rationale for each finding code, printed by
// `unagi vet --explain CODE` in the style of go vet and rustc's --explain: what
// the pattern is, why it was safe under the GIL and is not now, and the fix.
var explanations = map[string]string{
	"UNA-THR-001": unaThr001Explain,
	"UNA-THR-002": unaThr002Explain,
	"UNA-THR-003": unaThr003Explain,
	"UNA-THR-004": unaThr004Explain,
	"UNA-THR-005": unaThr005Explain,
	"UNA-THR-006": unaThr006Explain,
	"UNA-THR-007": unaThr007Explain,
	"UNA-THR-008": unaThr008Explain,
	"UNA-AIO-001": unaAio001Explain,
	"UNA-AIO-002": unaAio002Explain,
	"UNA-AIO-003": unaAio003Explain,
	"UNA-MP-001":  unaMp001Explain,
	"UNA-MP-002":  unaMp002Explain,
}

// Explain returns the long-form text for a finding code and whether the code is
// known.
func Explain(code string) (string, bool) {
	text, ok := explanations[code]
	return text, ok
}

const unaThr001Explain = `UNA-THR-001: unsynchronized read-modify-write

A read-modify-write reads a value, changes it, and stores the result back, which
is three steps. Under CPython's GIL those steps could not interleave with another
thread, so ` + "`counter += 1`" + ` behaved as if it were atomic. Without the GIL,
and on unagi, two threads can read the same value, each add one, and each store,
so one update is lost.

    counter = 0

    def worker():
        global counter
        for _ in range(100_000):
            counter += 1          # racy: read, add, store

The value stays a well-formed int and nothing crashes, but the final total is
smaller than it should be. Guard the update with a lock so the three steps run as
one:

    lock = threading.Lock()

    def worker():
        global counter
        for _ in range(100_000):
            with lock:
                counter += 1

The faster fix needs no lock at all: give each worker a private counter and
combine the results after join, so the threads never share a mutable cell.

    def worker(out, i):
        total = 0
        for _ in range(100_000):
            total += 1
        out[i] = total            # one writer per slot, summed after join
`

const unaThr002Explain = `UNA-THR-002: check-then-act race

A check-then-act tests a shared object and then, in a separate step, acts on
that observation. Under the GIL the two steps could not interleave, so the
observation was still true when the action ran. Without the GIL another thread
can change the object in the gap, so both threads pass the check and both act.

    cache = {}

    def get(key):
        if key not in cache:      # check
            cache[key] = build(key)   # act on a now-stale observation
        return cache[key]

Two threads can both find key missing, both call build, and the second store
overwrites the first, so the work is done twice and one result is discarded. The
lazy-init shape has the same window:

    if conn is None:
        conn = connect()          # two threads open two connections

Hold a lock across both halves so the check and the act are one step:

    with lock:
        if key not in cache:
            cache[key] = build(key)

Where the primitive offers it, an atomic operation closes the window without a
lock: dict.setdefault performs the test and the insert in one call.

    cache.setdefault(key, build(key))
`

const unaThr003Explain = `UNA-THR-003: shared-container iteration

Iterating a container walks it element by element, holding an internal cursor.
If another thread adds or removes items during the walk, the cursor no longer
matches the container. Under the GIL CPython noticed the size change and raised
RuntimeError; without it the loop can skip elements, see one twice, or read
freed memory.

    items = []

    def consume():
        for x in items:          # a producer thread is appending to items
            handle(x)

Iterating a dict view, ` + "`for k in d.keys()`" + `, has the same problem, since the
view is live. Iterate a snapshot taken before the loop so a later mutation
cannot disturb the cursor:

    for x in list(items):
        handle(x)

Or hold the lock that the writer also takes, across the whole loop, so no
mutation can land while the walk is in progress:

    with lock:
        for x in items:
            handle(x)
`

const unaThr004Explain = `UNA-THR-004: GIL-relict spin-wait

A spin-wait polls a shared flag in a loop that does no other work, waiting for
another thread to flip it:

    done = False

    def worker():
        global done
        run()
        done = True

    while not done:       # spin, or time.sleep(0.01) to poll
        pass

Under the GIL this happened to work. The interpreter released and reacquired the
GIL every few bytecodes, so the waiting thread kept re-reading done and the
worker's write became visible on the next switch. Without the GIL the loop pins
a CPU core re-reading an unsynchronized flag, and nothing guarantees the write
is ever observed.

Use a threading.Event, whose wait blocks until set with no polling and no busy
loop:

    done = threading.Event()

    def worker():
        run()
        done.set()

    done.wait()

For a value handed from one thread to another rather than a bare flag, a
queue.Queue or a Condition carries both the data and the wakeup.
`

const unaThr005Explain = `UNA-THR-005: lock acquired without try/finally

A lock acquired by hand must be released on every path out of the critical
section, including the one an exception takes. When the release rides the happy
path only, a raise in between leaks the lock:

    lock.acquire()
    do_work()             # if this raises, the next line never runs
    lock.release()

The lock stays held, and every other thread that waits on it blocks forever. The
manual fix is a try/finally, so the release runs however the block exits:

    lock.acquire()
    try:
        do_work()
    finally:
        lock.release()

The with statement is the same thing without the boilerplate, and is the form to
prefer:

    with lock:
        do_work()

This check leaves a try-lock such as ` + "`if lock.acquire(timeout=1):`" + ` alone,
since that form deliberately inspects the return value rather than blocking.
`

const unaThr006Explain = `UNA-THR-006: reliance on prompt finalization

CPython frees an object the instant its last reference goes away, by reference
count. Code often leaned on that to close a file without saying so:

    data = open(path).read()      # closed the moment read() returns
    for line in open(path):       # closed when the loop ends
        ...

The file object has no remaining reference after the statement, so CPython
closed the descriptor right away. unagi runs on Go's garbage collector, which
frees objects at some later collection, not at the last reference. The
descriptor stays open until then, and a thread that opens many files in a loop
can exhaust the file table before a collection runs.

Use a with block, which closes the file at the end of the statement regardless
of the collector:

    with open(path) as f:
        data = f.read()

The same holds for any object whose cleanup you were leaving to the refcount,
such as a socket or a lock; bind it and close it, or manage it with with.
`

const unaThr007Explain = `UNA-THR-007: daemon thread holding a resource

A daemon thread does not keep the process alive. When the main thread exits, the
interpreter kills every daemon where it stands, without unwinding its stack. No
finally runs, no with block closes, and no buffer is flushed.

    def log_forever():
        with open("out.log", "a") as f:
            while True:
                f.write(next_line())

    threading.Thread(target=log_forever, daemon=True).start()

Whatever the daemon had written into a userspace buffer but not yet flushed is
lost, and a file it was partway through is left truncated. The with block above
does not save it, because the abrupt kill never reaches the block's exit.

If the work must complete, run a non-daemon thread and join it, so the program
waits for it to finish:

    t = threading.Thread(target=log_forever)
    t.start()
    ...
    stop.set()
    t.join()

If the thread should stop when the program does, give it a shutdown Event it
checks each iteration, so it can flush and close before returning, and signal
that event before exit.
`

const unaThr008Explain = `UNA-THR-008: cross-tier sharing surprise

This one is specific to unagi, not to CPython. unagi compiles code in two tiers.
The static tier specializes a function on the types it can prove, and for a
module global with a type annotation it may read a typed shadow of the value
rather than the boxed cell every time.

    counter: int = 0        # annotation makes counter a typed-shadow candidate

    def bump():
        global counter
        counter = counter + 1

When another thread rebinds counter, unagi bumps a binding version and
eventually deoptimizes the static reader back to the boxed cell. But a reader
already part way through its specialized body can still use the old shadow, so
for a moment the two tiers disagree on the value. Plain CPython never shows this,
because it always reads the live global.

Two ways out. Keep the shared mutable state in a boxed container, a list or dict
cell, and guard every access with a lock, so there is one source of truth and no
per-tier shadow:

    state = {"counter": 0}

    def bump():
        with lock:
            state["counter"] += 1

Or, if the value really is a shared scalar counter, drop the annotation so unagi
never builds a typed shadow and the global stays boxed. A thread-mutated global
is usually the wrong place for a type annotation.
`

const unaAio001Explain = `UNA-AIO-001: blocking call in a coroutine

An event loop runs every task on a single thread and switches between them only
at an await. A coroutine that makes a blocking call never reaches an await while
that call is in flight, so no other task runs until it returns:

    async def handle(request):
        time.sleep(1)                 # the whole loop is frozen for a second
        data = requests.get(url)      # and again for the length of this request

One slow read can stall an entire server, because every other connection is
waiting behind it on the same thread. This was true under CPython too; without
the GIL nothing changes here, since the loop was always one thread.

Prefer an async equivalent that yields at the blocking point. Sleep with the
loop, not against it:

    await asyncio.sleep(1)

and reach for an async HTTP client, an async database driver, and so on, instead
of their blocking cousins. When no async version exists, push the blocking work
onto a worker thread so the loop stays free:

    data = await loop.run_in_executor(None, requests.get, url)

This check flags a recognized blocking primitive called straight inside an async
function. Passing the function as a value to run_in_executor is not a call, so
that offload form is not flagged.
`

const unaAio002Explain = `UNA-AIO-002: fire-and-forget task

asyncio.create_task schedules a coroutine and returns a Task. The event loop
keeps only a weak reference to that task, on the assumption that the caller
holds a strong one. When the caller drops it, the task can be collected before
it finishes:

    async def main():
        asyncio.create_task(worker())   # the returned task is kept nowhere
        await serve()

Two things can go wrong. The task may be garbage-collected mid-run and simply
stop, with no error. And if it raises, the exception is delivered to the loop's
default handler rather than to an awaiter, so it never surfaces where the work
was started. Both make bugs that appear only sometimes and are hard to trace.

Hold on to the task. Bind it and await it later:

    task = asyncio.create_task(worker())
    ...
    await task

or keep a set of tasks alive for as long as they need to run:

    background = set()
    t = asyncio.create_task(worker())
    background.add(t)
    t.add_done_callback(background.discard)

Cleanest of all, let an asyncio.TaskGroup own the task and await it at the end
of the block:

    async with asyncio.TaskGroup() as tg:
        tg.create_task(worker())

This check flags a bare asyncio.create_task whose result is discarded. A
create_task called on a TaskGroup is left alone, since the group holds it.
`

const unaAio003Explain = `UNA-AIO-003: loop-affinity violation

An asyncio event loop and the futures it watches are not thread-safe. They are
built to be touched by exactly one thread, the one running the loop. A worker
thread that reaches into them directly races the loop's internal bookkeeping:

    def worker(loop, fut):
        result = compute()
        loop.call_soon(fut.set_result, result)   # runs on the wrong thread

The one supported bridge from another thread is loop.call_soon_threadsafe. It
wakes the loop and hands it a callback to run on its own thread, so the loop
touches its own state and the future itself:

    def worker(loop, fut):
        result = compute()
        loop.call_soon_threadsafe(fut.set_result, result)

Note that fut.set_result is passed here as a value for the loop to call, not
called on the worker thread. That is the whole point: the completion happens on
the loop thread.

This check gates on a program that starts a thread, resolves the functions
handed to Thread(target=...), executor.submit, and loop.run_in_executor, and
flags a loop-affine call (call_soon, call_later, call_at, set_result,
set_exception) inside one of those bodies. The threadsafe form has a different
name and passes the completion as a value, so it is not flagged.
`

const unaMp001Explain = `UNA-MP-001: unsupported multiprocessing start method

A compiled unagi program is a single Go binary, and multiprocessing starts a
worker by re-executing that binary into a fresh process, reading its job from a
pipe by pickle. That is exactly CPython's spawn start method: a clean process
with no inherited interpreter state. spawn is the only method unagi supports.

fork and forkserver are not supported, and asking for them raises ValueError at
runtime:

    multiprocessing.set_start_method("fork")     # ValueError at run time
    ctx = multiprocessing.get_context("fork")    # same

Forking a multithreaded runtime is unsafe: the child gets one thread and a
scheduler frozen mid-step, so anything the other threads were holding, a lock, a
half-written buffer, is inherited broken. This is not unagi being fussy; CPython
3.14 flipped the Linux default away from fork for the same reason.

Use spawn, which unagi provides and which is now CPython's default too:

    multiprocessing.set_start_method("spawn")

or drop the explicit call and take the default. Because a worker is a fresh
process that reruns module top-level code, guard your entry point with
if __name__ == "__main__": exactly as CPython documents for spawn.

This check flags set_start_method and get_context called with fork or
forkserver, whether the method is passed positionally or as method=.
`

const unaMp002Explain = `UNA-MP-002: unpicklable worker target

multiprocessing runs a target in a fresh worker process, and it gets the target
there by pickling it and unpickling it on the far side. pickle stores a function
as a reference to its qualified name, module.qualname, which the worker looks up
in its own module table. Two common targets have no such name:

    multiprocessing.Process(target=lambda: work()).start()   # a lambda

    def outer():
        def job():                 # a closure, defined inside outer
            ...
        with multiprocessing.Pool() as pool:
            pool.map(job, items)   # also unpicklable

A lambda has no qualified name at all. A function defined inside another is not
reachable by name from the top of its module, and it may also close over local
variables that simply do not exist in the worker. Either way the parent raises
PicklingError when it tries to send the job, the same error CPython raises.

Give multiprocessing a module-level function, and pass anything it needs to
capture as plain arguments, which are pickled alongside it:

    def job(item, config):
        ...

    with multiprocessing.Pool() as pool:
        pool.starmap(job, [(item, config) for item in items])

This check flags a lambda or a provably nested function passed as target to
Process or as the callable to a Pool dispatch method (map, imap, starmap, apply,
and their async forms). A bound method is not flagged, since it pickles fine as
long as its object does.
`
