# User-defined __repr__, __str__, and __format__ drive how instances render
# through repr(), str(), print(), f-strings, format(), and containers.


class Vec:
    def __init__(self, x, y):
        self.x = x
        self.y = y

    def __repr__(self):
        return f"Vec({self.x}, {self.y})"


v = Vec(1, 2)
print(v)
print(repr(v))
print(str(v))
print([v, Vec(3, 4)])
print((v,))
print({"k": v})
print(f"here {v} and {v!r}")


class Point:
    def __init__(self, x, y):
        self.x = x
        self.y = y

    def __str__(self):
        return f"({self.x}, {self.y})"

    def __repr__(self):
        return f"Point(x={self.x}, y={self.y})"


p = Point(5, 6)
print(p)
print(repr(p))
print(str(p))
print([p])
print(f"str {p} repr {p!r}")


# __str__ inherited from a base, __repr__ only on the base.
class Base:
    def __repr__(self):
        return "Base.repr"


class Child(Base):
    pass


c = Child()
print(c)
print(repr(c))
print(str(c))


class Money:
    def __init__(self, cents):
        self.cents = cents

    def __format__(self, spec):
        dollars = self.cents / 100
        return format(dollars, spec if spec else ".2f")


m = Money(12345)
print(format(m, ""))
print(format(m, "10.1f"))
print(f"{m}")
print(f"{m:>12}")
print("cost is {}".format(m))


# A __format__ with no __str__: str() and print() fall back to object repr,
# but f"{m}" and format(m, "") go through __format__.
class OnlyFormat:
    def __format__(self, spec):
        return "FMT[" + spec + "]"


o = OnlyFormat()
print(format(o, ""))
print(format(o, "abc"))
print(f"{o}")
print(f"{o:xyz}")


# Error paths: a dunder that returns a non-string raises TypeError.
class BadRepr:
    def __repr__(self):
        return 42


class BadStr:
    def __str__(self):
        return 99


class BadFormat:
    def __format__(self, spec):
        return 7


try:
    repr(BadRepr())
except TypeError as e:
    print("TypeError: " + str(e))

try:
    str(BadStr())
except TypeError as e:
    print("TypeError: " + str(e))

try:
    format(BadFormat(), "")
except TypeError as e:
    print("TypeError: " + str(e))
