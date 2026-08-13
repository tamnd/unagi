# The builtin open is bound to _io, not builtins: its __module__ is _io, so
# CPython pickles it as the _io.open global rather than a builtins reference, and
# io.open is the very same object. This checks the raw bytes at each protocol, the
# round trip back to the one open function, memo sharing when it appears twice, and
# that io.open pickles to the same reference.
import pickle
import io


def show(label, fn):
    try:
        print(label, fn())
    except Exception as e:
        print(label, type(e).__name__, ":", e)


# Raw bytes: _io.open as a GLOBAL at protocols 2 and 3, STACK_GLOBAL at 4 and 5.
for proto in (2, 3, 4, 5):
    print("p%d" % proto, pickle.dumps(open, proto))

# Round trip: the reference loads back to the single open function object.
for proto in (2, 3, 4, 5):
    show("rt p%d" % proto, lambda proto=proto: pickle.loads(pickle.dumps(open, proto)) is open)

# open twice in a tuple: the second reference is memoized, and both come back as
# the same object.
print("tuple", pickle.dumps((open, open), 4))
back = pickle.loads(pickle.dumps((open, open), 4))
show("tuple rt", lambda: back[0] is open and back[1] is open and back[0] is back[1])

# io.open is bound to the same object, so it pickles to the same _io.open global
# and loads back to the builtin open.
show("io.open is open", lambda: io.open is open)
print("io.open bytes", pickle.dumps(io.open, 4))
show("io.open rt", lambda: pickle.loads(pickle.dumps(io.open, 4)) is open)

# open inside a list alongside a plain value keeps the reference and round-trips.
mixed = [open, 1, open]
data = pickle.dumps(mixed, 5)
print("mixed", data)
back = pickle.loads(data)
show("mixed rt", lambda: back[0] is open and back[1] == 1 and back[2] is open)
