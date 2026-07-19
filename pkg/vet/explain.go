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
