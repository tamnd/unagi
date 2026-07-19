import concurrent.futures
import threading

# A single worker keeps the run deterministic: the initializer fires once, then
# every task runs in submission order on that one thread.
data = threading.local()
init_calls = 0
init_lock = threading.Lock()


def init(prefix, start):
    global init_calls
    with init_lock:
        init_calls += 1
    data.value = prefix
    data.base = start


def work(n):
    return "%s-%d" % (data.value, data.base + n)


with concurrent.futures.ThreadPoolExecutor(
    max_workers=1, initializer=init, initargs=("worker", 100)
) as ex:
    results = list(ex.map(work, range(5)))

print("results", results)
print("init_calls", init_calls)

# submit sees the initialized worker-local state too
with concurrent.futures.ThreadPoolExecutor(
    max_workers=1, initializer=init, initargs=("solo", 0)
) as ex:
    fut = ex.submit(work, 7)
    print("submit", fut.result())

# a non-callable initializer is rejected at construction
try:
    concurrent.futures.ThreadPoolExecutor(initializer=42)
except TypeError as exc:
    print("TypeError", exc)

# initializer with no initargs runs with no arguments
flag = []


def note():
    flag.append("ran")


with concurrent.futures.ThreadPoolExecutor(max_workers=1, initializer=note) as ex:
    ex.submit(lambda: None).result()
print("flag", flag)
