import pickle
from collections import OrderedDict


def show(label, fn):
    try:
        print(label, repr(fn()))
    except Exception as e:
        print(label, type(e).__name__, ":", e)


def rt(x, proto):
    return pickle.loads(pickle.dumps(x, proto))


od = OrderedDict([("a", 1), ("b", 2), ("c", 3)])

# The reduction is the five-tuple (type, (), None, None, items iterator).
print("reduce len", len(od.__reduce__()))
print("reduce args", od.__reduce__()[1])
print("reduce state", od.__reduce__()[2])
print("reduce items", dict(od.__reduce__()[4]))

# A round-trip at every binary protocol comes back an equal OrderedDict.
for proto in (2, 3, 4, 5):
    show("rt %d" % proto, lambda p=proto: rt(od, p))
    show("rt type %d" % proto, lambda p=proto: type(rt(od, p)).__name__)

# Insertion order rides through, distinct from sorted order.
show("order", lambda: list(rt(OrderedDict([("z", 1), ("a", 2), ("m", 3)]), 5).keys()))

# An empty OrderedDict round-trips to an empty one.
show("empty items", lambda: list(rt(OrderedDict(), 5).items()))
show("empty type", lambda: type(rt(OrderedDict(), 5)).__name__)

# Nested mutables ride through and the copy is independent of the source.
src = OrderedDict([("x", [1, 2]), ("y", {"k": "v"})])
back = rt(src, 5)
print("nested", back)
src["x"].append(99)
print("nested independent", back["x"])

# A shared reference memoizes, so both come back the same object.
d = OrderedDict([("a", 1)])
pair = rt([d, d], 5)
print("shared", pair[0] is pair[1])

# __reduce_ex__ and __reduce__ report the same arity errors object does.
show("reduce_ex no arg", lambda: od.__reduce_ex__())
show("reduce extra arg", lambda: od.__reduce__(1))

# A move_to_end before pickling is reflected in the restored order.
mv = OrderedDict([("a", 1), ("b", 2), ("c", 3)])
mv.move_to_end("a")
show("move_to_end order", lambda: list(rt(mv, 5).keys()))
