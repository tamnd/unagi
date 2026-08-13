import pickle
import math


def show(label, fn):
    try:
        print(label, fn())
    except Exception as e:
        print(label, type(e).__name__, ":", e)


# A math function pickles as its math.<name> global, the same by-name form
# CPython derives from the function's __module__ and __qualname__. The raw bytes
# match at every protocol from 2 up.
for proto in (2, 3, 4, 5):
    for fn in (math.sqrt, math.log, math.gcd, math.comb, math.prod, math.isclose):
        show(("dumps", proto, fn.__name__), lambda fn=fn, proto=proto: pickle.dumps(fn, proto))

# The reference loads back to the very function object the module holds, so the
# round-trip is the same singleton and stays callable.
for proto in (2, 4, 5):
    back = pickle.loads(pickle.dumps(math.sqrt, proto))
    show(("roundtrip is", proto), lambda back=back: back is math.sqrt)
    show(("roundtrip call", proto), lambda back=back: back(144.0))

# The same function twice in a container shares one memo entry, so both slots
# load back to the one object.
shared = pickle.loads(pickle.dumps([math.sin, math.sin], 4))
show("memo bytes", lambda: pickle.dumps([math.sin, math.sin], 4))
show("memo shares", lambda: shared[0] is shared[1] and shared[0] is math.sin)

# A tuple of distinct functions round-trips element for element.
funcs = (math.floor, math.ceil, math.trunc)
show("tuple roundtrip", lambda: pickle.loads(pickle.dumps(funcs, 2)) == funcs)

# A module constant is a plain float, not a function, so it pickles as its own
# value and never names the math module.
show("pi bytes", lambda: pickle.dumps(math.pi, 2))
show("pi roundtrip", lambda: pickle.loads(pickle.dumps(math.pi, 5)) == math.pi)
