import threading

# threading.enumerate lists every live Thread object: the main thread plus every
# started thread that has not yet returned. CPython leaves the order unspecified,
# so this fixture only prints sorted names, membership, and counts, all of which
# are byte for byte stable against CPython.

# With no child started yet only the main thread is alive.
print("main only:", [t.name for t in threading.enumerate()] == ["MainThread"])
print("main in enum:", threading.main_thread() in threading.enumerate())
print("current in enum:", threading.current_thread() in threading.enumerate())
print("count matches:", threading.active_count() == len(threading.enumerate()))


def make_worker(k):
    def run():
        # The worker runs alone with the main thread, since each is joined before
        # the next starts, so the live set is exactly MainThread and this worker.
        print("worker", k, "sees:", sorted(t.name for t in threading.enumerate()))
        print("worker", k, "self in enum:", threading.current_thread() in threading.enumerate())
        print("worker", k, "count:", len(threading.enumerate()))

    return run


# Start and join each worker in turn so the output order is fixed and the live
# set inside each worker is deterministic.
threads = [threading.Thread(target=make_worker(k), name="W" + str(k)) for k in range(3)]
for k in range(3):
    threads[k].start()
    threads[k].join()

# Every worker has returned, so enumerate is back to the main thread alone.
print("after join:", [t.name for t in threading.enumerate()] == ["MainThread"])
print("final count:", len(threading.enumerate()))
