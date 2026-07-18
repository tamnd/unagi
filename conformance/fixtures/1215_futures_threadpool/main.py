import threading
from concurrent.futures import ThreadPoolExecutor

# submit hands back a Future whose result is the call's return value. The pool is
# its own context manager, shutting down on the way out of the with block.
with ThreadPoolExecutor(max_workers=2, thread_name_prefix="submit") as ex:
    print("typename:", type(ex).__name__)
    f = ex.submit(pow, 2, 10)
    print("submit result:", f.result())
    print("submit done:", f.done())

    def greet(name, greeting="hi"):
        return greeting + " " + name

    g = ex.submit(greet, "unagi", greeting="hey")
    print("submit kwargs:", g.result())

    def boom():
        raise ValueError("nope")

    b = ex.submit(boom)
    print("submit exc:", repr(b.exception()))

# A pool shut down by the with block refuses new work.
try:
    ex.submit(pow, 2, 2)
except RuntimeError as e:
    print("submit after shutdown:", e)

# map yields in submission order, zips several iterables, and stops at the
# shortest one.
with ThreadPoolExecutor(max_workers=4, thread_name_prefix="map") as ex:
    print("map:", list(ex.map(lambda x: x * x, [1, 2, 3, 4, 5])))
    print("map zip:", list(ex.map(lambda a, b: a + b, [1, 2, 3], [10, 20, 30])))
    print("map short:", list(ex.map(lambda a, b: (a, b), [1, 2, 3], [9, 8])))

    def half(x):
        return 10 // x

    got = []
    try:
        for v in ex.map(half, [1, 2, 0, 5]):
            got.append(v)
    except ZeroDivisionError as e:
        print("map raises:", e, "after", got)

# A worker is a threading.Thread named "<prefix>_<n>", so a one-worker pool runs
# every task on the same, deterministically named worker.
with ThreadPoolExecutor(max_workers=1, thread_name_prefix="solo") as ex:
    name1 = ex.submit(lambda: threading.current_thread().name).result()
    name2 = ex.submit(lambda: threading.current_thread().name).result()
    print("worker name:", name1)
    print("same worker:", name1 == name2)

# The default prefix is "ThreadPoolExecutor-N"; this is the first default-named
# pool in the program, so it is pool 0 and its lone worker is _0.
with ThreadPoolExecutor(max_workers=1) as ex:
    print("default worker name:", ex.submit(lambda: threading.current_thread().name).result())

# shutdown(cancel_futures=True) cancels the futures still queued behind the
# running one. A one-worker pool with a blocked first task keeps the rest queued.
started = threading.Event()
release = threading.Event()


def block():
    started.set()
    release.wait()
    return "ran"


ex = ThreadPoolExecutor(max_workers=1, thread_name_prefix="cancel")
running = ex.submit(block)
started.wait()
queued1 = ex.submit(lambda: 1)
queued2 = ex.submit(lambda: 2)
ex.shutdown(wait=False, cancel_futures=True)
print("cancelled queued:", queued1.cancelled(), queued2.cancelled())
release.set()
print("running finished:", running.result())
ex.shutdown()

# max_workers must be a positive number.
try:
    ThreadPoolExecutor(max_workers=0)
except ValueError as e:
    print("zero workers:", e)
try:
    ThreadPoolExecutor(max_workers=-3)
except ValueError as e:
    print("negative workers:", e)
