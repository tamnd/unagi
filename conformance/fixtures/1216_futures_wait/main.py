import threading
from concurrent.futures import ThreadPoolExecutor, wait, as_completed
from concurrent.futures import FIRST_COMPLETED, FIRST_EXCEPTION, ALL_COMPLETED

# The three return_when constants are the plain strings the module binds.
print("constants:", FIRST_COMPLETED, FIRST_EXCEPTION, ALL_COMPLETED)

# wait with the default ALL_COMPLETED blocks until every future is done, then
# returns the DoneAndNotDoneFutures namedtuple of two sets. A one-worker pool
# runs the tasks in turn, so the wait ends with all of them finished.
with ThreadPoolExecutor(max_workers=1, thread_name_prefix="all") as ex:
    fs = [ex.submit(pow, i, 2) for i in range(4)]
    res = wait(fs)
    print("result type:", type(res).__name__)
    print("result fields:", res._fields)
    print("all done:", len(res.done), "not done:", len(res.not_done))
    print("all results:", sorted(f.result() for f in res.done))

# FIRST_COMPLETED returns as soon as one future finishes. The first task returns
# at once; the second blocks on an event, so on a one-worker pool it has not even
# started when the wait returns. The done set holds exactly the first future.
with ThreadPoolExecutor(max_workers=1, thread_name_prefix="first") as ex:
    hold = threading.Event()
    fast = ex.submit(lambda: "fast")
    slow = ex.submit(lambda: hold.wait())
    res = wait([fast, slow], return_when=FIRST_COMPLETED)
    print("first done:", len(res.done), "first not done:", len(res.not_done))
    print("first has fast:", fast in res.done, "not slow:", slow in res.not_done)
    hold.set()

# FIRST_EXCEPTION returns as soon as a future finishes with an exception. The
# first task raises; the second blocks, so the wait returns holding the raiser.
with ThreadPoolExecutor(max_workers=1, thread_name_prefix="exc") as ex:
    hold = threading.Event()

    def boom():
        raise ValueError("boom")

    bad = ex.submit(boom)
    slow = ex.submit(lambda: hold.wait())
    res = wait([bad, slow], return_when=FIRST_EXCEPTION)
    print("exc done:", len(res.done), "exc not done:", len(res.not_done))
    print("exc raised:", repr(res.done.pop().exception()) if len(res.done) == 1 else "?")
    hold.set()

# A timeout returns whatever is done when it elapses. A blocked task is still
# pending after a short wait, so the done set is empty and it lands in not_done.
with ThreadPoolExecutor(max_workers=1, thread_name_prefix="tmo") as ex:
    hold = threading.Event()
    f = ex.submit(lambda: hold.wait())
    res = wait([f], timeout=0.05)
    print("timeout done:", len(res.done), "timeout not done:", len(res.not_done))
    hold.set()
    print("after release:", f.result())

# An invalid return_when raises ValueError, but only while a future is pending;
# an all-done group short-circuits before the condition is ever checked.
with ThreadPoolExecutor(max_workers=1, thread_name_prefix="bad") as ex:
    hold = threading.Event()
    pending = ex.submit(lambda: hold.wait())
    try:
        wait([pending], return_when="BOGUS")
    except ValueError as e:
        print("invalid return_when:", e)
    hold.set()
    done = ex.submit(lambda: 1)
    done.result()
    res = wait([done], return_when="BOGUS")
    print("invalid but all done:", len(res.done))

# wait rejects a non-future the way CPython does when it reaches for the future
# machinery the element does not have.
try:
    wait([1])
except AttributeError as e:
    print("non-future:", e)

# as_completed yields the futures in the order they finish. A one-worker pool
# finishes them in submission order, so the results read back in that order.
with ThreadPoolExecutor(max_workers=1, thread_name_prefix="ac") as ex:
    fs = [ex.submit(pow, 2, i) for i in range(5)]
    got = [f.result() for f in as_completed(fs)]
    print("as_completed:", got)

# as_completed deduplicates the futures it is handed, yielding each once.
with ThreadPoolExecutor(max_workers=1, thread_name_prefix="dup") as ex:
    f = ex.submit(lambda: 99)
    got = [x.result() for x in as_completed([f, f, f])]
    print("as_completed dedup:", got)

# as_completed raises TimeoutError when its deadline passes with futures still
# pending, naming how many of the total are unfinished.
with ThreadPoolExecutor(max_workers=1, thread_name_prefix="actmo") as ex:
    hold = threading.Event()
    f = ex.submit(lambda: hold.wait())
    try:
        for _ in as_completed([f], timeout=0.05):
            pass
    except TimeoutError as e:
        print("as_completed timeout:", e)
    hold.set()
