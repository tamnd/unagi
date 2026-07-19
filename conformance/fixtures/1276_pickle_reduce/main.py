import pickle

# A class that defines __reduce__ or __reduce_ex__ pickles through the reduction
# tuple it returns instead of the default NEWOBJ path: the callable it names goes
# out as a global reference, its argument tuple follows, and REDUCE applies the
# one to the other on load. A third element is the object state, saved after
# REDUCE and applied by BUILD. The bytes are observable, so the global, the
# argument tuple, the REDUCE, and any trailing state must match CPython slot for
# slot.


def rebuild(x, y):
    return Coord(x, y)


class Coord:
    def __init__(self, x, y):
        self.x = x
        self.y = y

    def __eq__(self, other):
        return isinstance(other, Coord) and self.x == other.x and self.y == other.y

    def __repr__(self):
        return f"Coord({self.x}, {self.y})"

    def __reduce__(self):
        # A plain two-element reduction: rebuild(self.x, self.y) reconstructs it.
        return (rebuild, (self.x, self.y))


def make_box(a):
    return Box(a, None)


class Box:
    def __init__(self, a, b):
        self.a = a
        self.b = b

    def __eq__(self, other):
        return isinstance(other, Box) and self.a == other.a and self.b == other.b

    def __repr__(self):
        return f"Box({self.a!r}, {self.b!r})"

    def __reduce__(self):
        # A three-element reduction: make_box builds a partial object and the state
        # dict fills in the rest through BUILD.
        return (make_box, (self.a,), {"b": self.b})


class Temperature:
    def __init__(self, celsius):
        self.celsius = celsius

    def __eq__(self, other):
        return isinstance(other, Temperature) and self.celsius == other.celsius

    def __repr__(self):
        return f"Temperature({self.celsius})"

    def __reduce_ex__(self, protocol):
        # __reduce_ex__ takes the protocol and reconstructs through the class itself.
        return (Temperature, (self.celsius,))


def make_ledger():
    return Ledger(0)


class Ledger:
    def __init__(self, balance):
        self.balance = balance
        self.log = []

    def __eq__(self, other):
        return isinstance(other, Ledger) and self.balance == other.balance and self.log == other.log

    def __repr__(self):
        return f"Ledger({self.balance}, {self.log})"

    def __reduce__(self):
        return (make_ledger, (), {"balance": self.balance, "log": self.log})

    def __setstate__(self, state):
        # A custom __setstate__ receives the BUILD state and restores the instance
        # however it likes, here rebuilding the log as a fresh list.
        self.balance = state["balance"]
        self.log = list(state["log"])


# A plain two-element reduction round-trips through rebuild().
for proto in (2, 3, 4, 5):
    c = Coord(3, 4)
    data = pickle.dumps(c, protocol=proto)
    print("coord", proto, data.hex(), pickle.loads(data) == c)

# A three-element reduction saves state after REDUCE and applies it with BUILD.
for proto in (2, 3, 4, 5):
    b = Box("hi", [1, 2, 3])
    data = pickle.dumps(b, protocol=proto)
    print("box", proto, data.hex(), pickle.loads(data) == b)

# __reduce_ex__ takes the protocol and reconstructs through the class object.
for proto in (2, 3, 4, 5):
    t = Temperature(21)
    data = pickle.dumps(t, protocol=proto)
    print("temp", proto, data.hex(), pickle.loads(data) == t)

# A class with __setstate__ restores itself from the BUILD state through the hook.
for proto in (2, 3, 4, 5):
    ledger = Ledger(100)
    ledger.log = ["open", "deposit"]
    data = pickle.dumps(ledger, protocol=proto)
    print("ledger", proto, data.hex(), pickle.loads(data) == ledger)
