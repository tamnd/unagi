# The slice builtin: constructor, attributes, repr, equality, hashing,
# type identity, use as a subscript key and slice.indices.
s = slice(1, 10, 2)
print(repr(s), s.start, s.stop, s.step)
print(repr(slice(5)), repr(slice(1, 2)))
print(slice(None, None, -1))

# type identity and membership
print(type(slice(1, 2)))
print(type(slice))
print(slice.__name__)
print(isinstance(slice(1, 2), slice))
print(isinstance(5, slice))
print(issubclass(slice, object), issubclass(slice, slice))

# equality and hashing key equal slices together
print(slice(1, 10, 2) == slice(1, 10, 2), slice(1, 2) == slice(1, 3))
print(slice(1, 2) == 5, slice(1, 2) != slice(1, 2))
d = {slice(1, 2, 3): "a", slice(4, 5): "b"}
print(d[slice(1, 2, 3)], d[slice(4, 5)])

# non-int bounds round-trip through the parts
odd = slice(1.5, "x", None)
print(odd.start, repr(odd.stop), odd.step)

# indices resolves against a length
print(slice(None, 5, None).indices(20))
print(slice(1, 10, 2).indices(20))
print(slice(None, None, -1).indices(5))
print(slice(-1, -5, -1).indices(10))

# a slice object subscripts the builtin sequences like the bracket notation
xs = [10, 20, 30, 40, 50]
print(xs[slice(1, 4)], xs[slice(None, None, 2)])
print((1, 2, 3, 4)[slice(None, None, -1)])
print("hello"[slice(1, 4)])
ys = [0, 1, 2, 3, 4, 5]
ys[slice(0, 2)] = [9, 9, 9]
print(ys)
del ys[slice(None, None, 2)]
print(ys)

# a user class receives the slice object through __getitem__
class Grab:
    def __getitem__(self, k):
        return k
print(repr(Grab()[1:10:2]))
print(repr(Grab()[::-1]))
print(repr(Grab()[:]))

# match on the slice type
def kind(v):
    match v:
        case slice():
            return "slice"
        case _:
            return "other"
print(kind(slice(1, 2)), kind(5))

# arity and indices errors
for bad in (lambda: slice(),
            lambda: slice(1, 2, 3, 4),
            lambda: slice(1, 2).indices(-5),
            lambda: slice(1, 2).indices("x"),
            lambda: slice(1, 2).indices(1, 2)):
    try:
        bad()
    except (TypeError, ValueError) as e:
        print(type(e).__name__, e)
