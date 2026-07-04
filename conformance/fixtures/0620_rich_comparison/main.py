# User-defined rich comparison dunders drive ==, !=, <, <=, >, >= on instances,
# with reflected-operand fallback, __ne__ derived from __eq__, subclass priority,
# and the unorderable TypeError when no ordering method applies.


class Num:
    def __init__(self, v):
        self.v = v

    def __eq__(self, o):
        if isinstance(o, Num):
            return self.v == o.v
        return NotImplemented

    def __lt__(self, o):
        if isinstance(o, Num):
            return self.v < o.v
        return NotImplemented


a, b, c = Num(3), Num(3), Num(5)
print(a == b, a == c)
print(a != b, a != c)
print(a < c, c < a)
# __eq__ declines for a non-Num, so both sides fall back to identity.
print(a == "x", "x" == a, a != "x")


# An instance whose __eq__ handles a plain value compares through reflection too.
class Val:
    def __init__(self, v):
        self.v = v

    def __eq__(self, o):
        other = o.v if isinstance(o, Val) else o
        return self.v == other


print(Val(3) == 3, 3 == Val(3))
print(Val(3) == Val(3), Val(3) == 4)


# A class with the full ordering set sorts and compares every which way.
class Ver:
    def __init__(self, n):
        self.n = n

    def __eq__(self, o):
        return self.n == o.n

    def __lt__(self, o):
        return self.n < o.n

    def __le__(self, o):
        return self.n <= o.n

    def __gt__(self, o):
        return self.n > o.n

    def __ge__(self, o):
        return self.n >= o.n

    def __repr__(self):
        return f"Ver({self.n})"


p, q = Ver(1), Ver(2)
print(p < q, p <= q, p > q, p >= q, p <= Ver(1))
print(sorted([Ver(3), Ver(1), Ver(2)]))


# No __eq__ at all: identity equality, and ordering is a TypeError.
class Bare:
    pass


x = Bare()
y = Bare()
print(x == x, x == y, x != y)
try:
    x < y
except TypeError as e:
    print("TypeError: " + str(e))


# A subclass that overrides the reflected slot answers before the base.
class Base:
    def __eq__(self, o):
        print("Base.__eq__ ran")
        return NotImplemented


class Sub(Base):
    def __eq__(self, o):
        print("Sub.__eq__ ran")
        return True


print(Base() == Sub())


# An explicit __ne__ wins over the derived negation of __eq__.
class Weird:
    def __eq__(self, o):
        return True

    def __ne__(self, o):
        return True


w = Weird()
print(w == w, w != w)


# An exception raised inside a comparison dunder propagates.
class Boom:
    def __lt__(self, o):
        raise ValueError("no order")


try:
    Boom() < Boom()
except ValueError as e:
    print("ValueError: " + str(e))
