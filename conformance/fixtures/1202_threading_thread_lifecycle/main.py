import threading

# A worker prints its label; each thread is started then joined right away, so
# the interleaving of thread output with main output is fixed and the golden is
# deterministic. The threads still run on their own goroutines, spawned and
# joined through the real thread machinery.
def worker(label):
    def run():
        print("run", label)
    return run

threads = []
for k in range(3):
    threads.append(threading.Thread(target=worker(k)))

idents = []
for t in threads:
    print(t.is_alive())
    t.start()
    t.join()
    print(t.is_alive())
    idents.append(t.ident)

# Recycle-agnostic observables: CPython may reuse a finished thread's ident, so
# the fixture never asserts the idents are distinct, only that each is an int
# and none collides with the still-alive main thread.
print(all(isinstance(i, int) for i in idents))
print(all(i != threading.get_ident() for i in idents))

# The default name carries the "Thread-N (target)" shape, N assigned in the
# order the unnamed threads were built.
def compute():
    pass
named = threading.Thread(target=compute)
print(named.name)
print(named.daemon)

# current_thread and main_thread agree on the main thread here.
print(threading.current_thread().name)
print(threading.current_thread() is threading.main_thread())
