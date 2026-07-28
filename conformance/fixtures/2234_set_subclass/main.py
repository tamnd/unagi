# Subclassing the builtin set and frozenset: construction, inherited methods,
# the set operators returning the base type, membership, iteration and repr,
# plus a super() chain through an overridden method.


class S(set):
    pass


s = S([1, 2, 3])
print(len(s))
print(2 in s, 9 in s)
s.add(4)
s.discard(1)
print(sorted(s))
print(isinstance(s, set), isinstance(s, frozenset))
print(repr(S()))
print(repr(S([1, 2])))

u = s | {10}
print(sorted(u), type(u).__name__)
d = s - {2}
print(sorted(d), type(d).__name__)
print(sorted(s & {3, 4, 99}))
print(sorted(s ^ {3, 100}))
print(s == {2, 3, 4})
print({2, 3} <= s)
print(s <= {2, 3})

un = s.union({50})
print(sorted(un), type(un).__name__)

s.clear()
print(len(s), list(s))


class F(frozenset):
    pass


f = F([1, 2])
print(repr(f), len(f), 1 in f)
ff = f | {3}
print(sorted(ff), type(ff).__name__)
print(f == frozenset([1, 2]), f == {1, 2})


class Counting(set):
    def __init__(self, *a):
        super().__init__(*a)
        self.adds = 0

    def add(self, x):
        self.adds += 1
        super().add(x)


c = Counting([1])
c.add(2)
c.add(3)
print(sorted(c), c.adds)
