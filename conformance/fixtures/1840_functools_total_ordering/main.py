# functools.total_ordering, the class decorator that fills in the ordering
# methods a class leaves out from the one it defines. Each derived method calls
# the root operation and, unless that returns NotImplemented, combines the result
# with an equality check. A class with no ordering operation raises ValueError,
# and a root that returns NotImplemented against a foreign type propagates so the
# usual unorderable-types TypeError still fires.
import functools
from functools import total_ordering


@total_ordering
class Num:
    def __init__(self, v):
        self.v = v

    def __eq__(self, other):
        if not isinstance(other, Num):
            return NotImplemented
        return self.v == other.v

    def __lt__(self, other):
        if not isinstance(other, Num):
            return NotImplemented
        return self.v < other.v

    def __repr__(self):
        return "Num(%d)" % self.v


a, b, c = Num(1), Num(2), Num(1)
print(a < b, a <= b, a > b, a >= b)
print(a <= c, a >= c, a == c)
print(b > a, b >= a)
print(a != b, a != c)
print(sorted([Num(3), Num(1), Num(2), Num(1)]))


@total_ordering
class FromGt:
    def __init__(self, v):
        self.v = v

    def __eq__(self, o):
        return self.v == o.v

    def __gt__(self, o):
        return self.v > o.v


x, y = FromGt(5), FromGt(3)
print(x > y, x < y, x >= y, x <= y)


@total_ordering
class FromLe:
    def __init__(self, v):
        self.v = v

    def __eq__(self, o):
        return self.v == o.v

    def __le__(self, o):
        return self.v <= o.v


p, q = FromLe(2), FromLe(4)
print(p < q, p > q, p >= q, p <= q)


def show(fn):
    try:
        return fn()
    except Exception as e:
        return type(e).__name__ + " " + str(e)


# A root that returns NotImplemented against a foreign type leaves the derived
# operation unorderable, the same TypeError as an undecorated class.
print(show(lambda: Num(1) > "x"))


class Bad:
    def __eq__(self, o):
        return True


# total_ordering applied to a class with no ordering operation raises ValueError.
print(show(lambda: total_ordering(Bad)))
