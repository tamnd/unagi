import atexit


def bye(name):
    print("bye", name)


# register returns the function, so it works plain and as a decorator, and the
# callbacks run last-in-first-out.
atexit.register(bye, "first")
atexit.register(bye, "second")


@atexit.register
def decorated():
    print("decorated")


print("count", atexit._ncallbacks())


# unregister drops every entry for a function; a function never registered is a
# no-op.
def temp():
    print("temp should not run")


atexit.register(temp)
print("with temp", atexit._ncallbacks())
atexit.unregister(temp)
atexit.unregister(bye)  # removes both bye entries
print("after unregister", atexit._ncallbacks())


# _run_exitfuncs runs and clears the pending callbacks right here, deterministic
# in the body, so only the decorated one is left (bye entries were removed).
atexit._run_exitfuncs()
print("after run", atexit._ncallbacks())


# A callback registered now runs at real interpreter exit, after the body.
atexit.register(bye, "at-exit")
print("body done")
