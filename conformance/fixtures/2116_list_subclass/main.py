# Subclassing the builtin list: a class may name list among its bases and its
# instances behave as lists through the inherited sequence protocol and methods,
# while a user override still wins. This is the list layer of the builtin
# subclassing frontier, the shape importlib._bootstrap's class _List(list) needs.


class L(list):
    pass


# Construction from an iterable and empty.
a = L([1, 2, 3])
print(a, type(a).__name__, len(a))
print(L(), len(L()))

# Indexing and slicing read through to the payload.
print(a[0], a[-1], a[1:3])

# Item assignment and deletion mutate the payload.
a[0] = 99
print(a)
del a[0]
print(a)

# Inherited list methods bind to the instance store.
a.append(7)
a.extend([8, 9])
a.insert(0, 0)
print(a)
print(a.count(7), a.index(8))
a.sort()
print(a)
a.reverse()
print(a)
c = a.copy()
print(c, type(c).__name__)

# Iteration and membership.
print([x for x in L([4, 5, 6])])
print(5 in L([4, 5, 6]), 9 in L([4, 5, 6]))

# Concatenation and repetition return a plain list.
print(L([1, 2]) + [3, 4], type(L([1, 2]) + [3, 4]).__name__)
print(L([1]) * 3, type(L([1]) * 3).__name__)
print(2 * L([1]))

# Comparison runs element by element against lists and other subclasses.
print(L([1, 2]) == [1, 2], L([1, 2]) == L([1, 2]))
print(L([1, 2]) < L([1, 3]), L([1, 2, 3]) < L([1, 2]))

# repr borrows list's bracket body with no class name.
print(repr(L([1, 2, 3])))

# Augmented assignment mutates in place and keeps the subclass type.
d = L([1, 2])
d += [3, 4]
print(d, type(d).__name__)
e = L([1, 2])
e *= 2
print(e, type(e).__name__)


# super().__init__ reaches the inherited list initializer.
class M(list):
    def __init__(self, n):
        super().__init__(range(n))


print(M(4), type(M(4)).__name__)


# A user __getitem__ override wins over the inherited one.
class N(list):
    def __getitem__(self, i):
        return "got"


print(N([1, 2])[0])

# Truthiness follows length.
print(bool(L()), bool(L([1])))
