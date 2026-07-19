import pickle

# A plain user-defined class instance pickles through CPython's default object
# reduction: (copyreg.__newobj__, (cls,), state). The stream names the class by
# its module and qualname, builds a bare instance with NEWOBJ, and, when the
# instance carries attributes, restores its __dict__ with BUILD. The exact bytes
# are observable, so the class name, the attribute order, and the presence or
# absence of BUILD must all match CPython. Each class defines __eq__ so a
# round-trip compares by value rather than identity.


class Point:
    def __init__(self, x, y):
        self.x = x
        self.y = y

    def __eq__(self, other):
        return isinstance(other, Point) and self.x == other.x and self.y == other.y


class Empty:
    def __eq__(self, other):
        return isinstance(other, Empty)


class Box:
    def __init__(self, value):
        self.value = value

    def __eq__(self, other):
        return isinstance(other, Box) and self.value == other.value


# An instance with attributes carries a BUILD; an attribute-free instance has
# state None and stops right after NEWOBJ. The attribute order is the __dict__
# insertion order, which the pickle preserves slot for slot.
values = [
    Point(1, "hi"),
    Point(-5, 12345),
    Empty(),
    Box([1, 2, 3]),
    Box({"a": 1, "b": 2}),
    Box(Point(7, 8)),
]
for v in values:
    data = pickle.dumps(v)
    print(data.hex(), pickle.loads(data) == v)

# The same instance shared through a container is pickled once and fetched back
# by memo, so both slots recover the identical object.
p = Point(3, 4)
shared = pickle.dumps([p, p])
sb = pickle.loads(shared)
print("shared:", shared.hex(), sb[0] is sb[1], sb[0] == p)

# An instance nested in a dict keeps its own reduction inside the container. The
# reconstructed instances are compared one at a time, so the round-trip check
# goes through each class's __eq__ rather than a container comparison.
nested = pickle.dumps({"pt": Point(0, 0), "box": Box(9)})
loaded = pickle.loads(nested)
print("nested:", nested.hex(), loaded["pt"] == Point(0, 0), loaded["box"] == Box(9))

# Every binary protocol reduces the instance the same way, differing only in the
# GLOBAL vs STACK_GLOBAL framing and the memo opcodes, and each round-trips.
for proto in (2, 3, 4, 5):
    v = Point(11, 22)
    data = pickle.dumps(v, protocol=proto)
    print("proto", proto, data.hex(), pickle.loads(data) == v)

# An attribute-free instance stops right after NEWOBJ under every protocol, with
# no BUILD.
for proto in (2, 3, 4, 5):
    data = pickle.dumps(Empty(), protocol=proto)
    print("empty proto", proto, data.hex(), pickle.loads(data) == Empty())
