import pickle

# A class defining __getnewargs_ex__ returns (args, kwargs) that NEWOBJ_EX feeds
# cls.__new__(cls, *args, **kwargs), so a class whose __new__ takes keyword-only
# constructor values is reconstructed with them. The argument tuple and keyword
# dict land right before NEWOBJ_EX, so their bytes are observable and must match
# CPython slot for slot. NEWOBJ_EX is a protocol-4 opcode; the default protocol is
# 5, so this covers the protocols that carry it.


class Point:
    def __new__(cls, x, *, y):
        self = object.__new__(cls)
        self.x = x
        self.y = y
        return self

    def __eq__(self, other):
        return isinstance(other, Point) and self.x == other.x and self.y == other.y

    def __repr__(self):
        return f"Point({self.x}, y={self.y})"

    def __getnewargs_ex__(self):
        return ((self.x,), {"y": self.y})

    def __getstate__(self):
        # The whole value rides in __new__, so BUILD is skipped.
        return None


class Config:
    # Positional and keyword arguments together, plus __dict__ state that BUILD
    # restores after NEWOBJ_EX rebuilds the keyword-configured shell. The keyword
    # is stored under a distinct attribute name and every value is an int, so no
    # string flows into both the NEWOBJ_EX arguments and the BUILD state, keeping
    # the bytes independent of CPython's cross-scope string interning.
    def __new__(cls, code, *, level=0):
        self = object.__new__(cls)
        self.code = code
        self.lvl = level
        return self

    def __init__(self, code, *, level=0):
        self.tag = 99

    def __eq__(self, other):
        return (
            isinstance(other, Config)
            and self.code == other.code
            and self.lvl == other.lvl
            and self.tag == other.tag
        )

    def __repr__(self):
        return f"Config(code={self.code}, lvl={self.lvl}, tag={self.tag})"

    def __getnewargs_ex__(self):
        return ((self.code,), {"level": self.lvl})


# NEWOBJ_EX carries the (args, kwargs) __getnewargs_ex__ produced; __getstate__
# returning None stops the pickle right after it.
for proto in (4, 5):
    p = Point(3, y=4)
    data = pickle.dumps(p, protocol=proto)
    print("point", proto, data.hex(), pickle.loads(data) == p)

# A class carrying __dict__ state pickles NEWOBJ_EX then BUILD.
for proto in (4, 5):
    c = Config(7, level=2)
    data = pickle.dumps(c, protocol=proto)
    print("config", proto, data.hex(), pickle.loads(data) == c)
