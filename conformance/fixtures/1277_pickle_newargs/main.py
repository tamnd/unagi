import pickle

# A class defining __getnewargs__ supplies the arguments NEWOBJ passes to
# cls.__new__(cls, *args), so a class whose __new__ needs its constructor values
# is reconstructed with them before BUILD restores the rest of __dict__. The
# argument tuple lands right after the class global and before NEWOBJ, so its
# bytes are observable and must match CPython slot for slot.


class Vec:
    def __new__(cls, x, y):
        self = object.__new__(cls)
        self.x = x
        self.y = y
        return self

    def __init__(self, x, y):
        self.x = x
        self.y = y

    def __eq__(self, other):
        return isinstance(other, Vec) and self.x == other.x and self.y == other.y

    def __repr__(self):
        return f"Vec({self.x}, {self.y})"

    def __getnewargs__(self):
        return (self.x, self.y)


class Frozen:
    # __new__ takes the value and stashes it; __getnewargs__ round-trips it, and
    # there is no extra __dict__ state, so the pickle stops right after NEWOBJ.
    def __new__(cls, value):
        self = object.__new__(cls)
        object.__setattr__(self, "value", value)
        return self

    def __eq__(self, other):
        return isinstance(other, Frozen) and self.value == other.value

    def __repr__(self):
        return f"Frozen({self.value!r})"

    def __getnewargs__(self):
        return (self.value,)

    def __getstate__(self):
        # No instance state beyond what __new__ rebuilds, so BUILD is skipped.
        return None


# NEWOBJ carries the __getnewargs__ tuple, then BUILD restores the __dict__.
for proto in (2, 3, 4, 5):
    v = Vec(3, 4)
    data = pickle.dumps(v, protocol=proto)
    print("vec", proto, data.hex(), pickle.loads(data) == v)

# A class whose whole value rides in __new__ pickles with NEWOBJ and no BUILD.
for proto in (2, 3, 4, 5):
    f = Frozen("hi")
    data = pickle.dumps(f, protocol=proto)
    print("frozen", proto, data.hex(), pickle.loads(data) == f)
