import threading

# The ambient thread now reaches the identity builtins, so get_ident and
# current_thread read the goroutine that actually runs the call rather than the
# main thread. Each worker observes its own identity from inside the child and
# main reads it back through the Thread object after the join, all byte for byte
# against CPython.

main_id = threading.get_ident()
main = threading.current_thread()
print("main name:", main.name)
print("main is main_thread:", main is threading.main_thread())


def make_worker(k):
    def run():
        me = threading.current_thread()
        print("worker", k, "name:", me.name)
        # get_ident inside the child equals the child Thread's ident, the way
        # CPython sets t.ident to the value get_ident returns on that thread.
        print("worker", k, "ident matches:", threading.get_ident() == me.ident)
        # and it is not the main thread's ident, since the call runs on the child.
        print("worker", k, "not main:", threading.get_ident() != main_id)
        # current_thread inside the child is the very Thread object we started.
        print("worker", k, "is own thread:", me is threads[k])

    return run


# Build every thread up front so a worker can compare current_thread() against
# its own object, then start and join each in turn so the output order is fixed.
threads = [threading.Thread(target=make_worker(k), name="W" + str(k)) for k in range(3)]
for k in range(3):
    threads[k].start()
    threads[k].join()

# With every worker joined only the main thread is left alive.
print("active after join:", threading.active_count())
