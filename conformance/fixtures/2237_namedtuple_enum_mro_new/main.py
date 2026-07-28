from collections import namedtuple
from enum import Enum

# ssl.py builds `class Purpose(_ASN1Object, _Enum)` at import, where _ASN1Object
# is a namedtuple with a custom __new__. The namedtuple base sits before Enum in
# the MRO, so the super().__new__ inside the namedtuple base must reach the
# tuple constructor and fill the fields, not Enum.__new__.
class Base(namedtuple("Base", "a b")):
    __slots__ = ()

    def __new__(cls, val):
        return super().__new__(cls, val, val * 2)


class Color(Base, Enum):
    X = 5
    Y = 7


print(Color.X.a, Color.X.b)
print(Color.Y.a, Color.Y.b)
# The members keep their tuple identity and namedtuple field access.
print(tuple(Color.X))
print(Color.X.a + Color.Y.b)
print(isinstance(Color.X, tuple))
print(Color.X.a == 5)

# A plain namedtuple subclass with a spelled-out super().__new__ outside any
# Enum still binds its fields positionally.
Point = namedtuple("Point", "x y")


class Shift(Point):
    __slots__ = ()

    def __new__(cls, x, y):
        return super().__new__(cls, x + 1, y + 1)


s = Shift(10, 20)
print(s.x, s.y)
print(s._replace(x=100).x, s._replace(x=100).y)
