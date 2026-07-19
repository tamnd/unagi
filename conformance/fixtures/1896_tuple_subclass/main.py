class MyTuple(tuple):
    pass


a = MyTuple([1, 2, 3])
b = MyTuple((4, 5))

# Length, indexing, slicing and membership read the payload.
print(len(a), a[0], a[-1], a[1:3], 2 in a, 9 in a)

# Iteration walks the elements in order.
print([x * 10 for x in a])

# Concatenation and repetition return a plain tuple; the subclass does not
# propagate.
print(a + b, type(a + b).__name__)
print(a * 2, type(a * 2).__name__)

# Comparison against plain tuples and other instances, both directions.
print(a == (1, 2, 3), (1, 2, 3) == a, a < b, a != b)

# isinstance, issubclass and the class identity.
print(isinstance(a, tuple), isinstance(a, MyTuple), issubclass(MyTuple, tuple), type(a).__name__)

# str and repr read through to the underlying tuple, and f-strings too.
print(str(a), repr(a), f"{a}")

# Hashing matches the underlying tuple, so instances key like their value.
print(hash(a) == hash((1, 2, 3)))
d = {MyTuple((1, 2)): "pair"}
print(d[(1, 2)])

# Inherited tuple methods run on the payload.
print(a.count(2), a.index(3), MyTuple([7, 7, 8]).count(7))


# A subclass that builds its payload in __new__ through tuple.__new__, the shape
# codecs.CodecInfo takes: extra constructor arguments become attributes.
class Pair(tuple):
    def __new__(cls, x, y, name=None):
        self = tuple.__new__(cls, (x, y))
        self.name = name
        return self

    def __repr__(self):
        return f"Pair({self[0]!r}, {self[1]!r}, name={self.name!r})"


p = Pair(1, 2, name="xy")
print(p, p[0], p[1], p.name)
print(str(p), len(p), isinstance(p, tuple))
print(p == (1, 2))


# A subclass reached through super().__new__ builds the payload the same way.
class Doubled(tuple):
    def __new__(cls, values):
        return super().__new__(cls, [v * 2 for v in values])


dbl = Doubled([1, 2, 3])
print(dbl, type(dbl).__name__, list(dbl))
