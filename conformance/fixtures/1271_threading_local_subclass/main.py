import threading

# A threading.local subclass keeps its instance attributes private to each
# thread while its class attributes and methods stay shared. __init__ runs on the
# constructing thread at creation, and again on any other thread the first time
# it touches the instance, re-applying the original constructor arguments. Each
# thread is started and joined in turn so the interleaved __init__ prints land in
# a fixed order, byte for byte against CPython.


class Store(threading.local):
    version = "v1"  # a shared class attribute, the same from every thread

    def __init__(self, tag):
        # Runs once per thread; the print makes the re-run on a new thread visible.
        print("init on", threading.current_thread().name, "tag", tag)
        self.tag = tag
        self.hits = 0

    def touch(self):
        # A shared method that reads and writes this thread's own attributes.
        self.hits += 1
        return self.tag, self.hits


store = Store("main")
print("main tag:", store.tag)
print("main version:", store.version)
print("main touch:", store.touch())
print("main touch:", store.touch())


def worker(k):
    # The first read re-runs __init__ with the stashed "main", giving this thread
    # its own fresh dict; the writes below never reach the main thread's copy.
    print("worker", k, "sees tag:", store.tag)
    print("worker", k, "version:", store.version)
    store.tag = "w" + str(k)
    print("worker", k, "touch:", store.touch())
    print("worker", k, "touch:", store.touch())


threads = [threading.Thread(target=worker, args=(k,), name="W" + str(k)) for k in range(2)]
for k in range(2):
    threads[k].start()
    threads[k].join()

# The main thread's attributes are untouched by the workers.
print("main tag after:", store.tag)
print("main touch after:", store.touch())

try:
    store.missing
except AttributeError as e:
    print("miss:", e)
