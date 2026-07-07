# collections.namedtuple, the factory that returns a tuple subclass whose fields
# are reachable by name. This exercises construction positionally and by keyword,
# field and index access, tuple behavior (equality with a bare tuple, unpacking),
# the _fields/_field_defaults/__name__ class data, the _asdict/_replace/_make
# helpers, defaults aligned to the trailing fields, rename fixing bad names, and
# the field-name validation errors.
import collections
from collections import namedtuple

Point = namedtuple("Point", ["x", "y"])
p = Point(1, 2)
print(p)
print(p.x, p.y, p[0], p[1])
print(p == (1, 2))
print(Point(y=5, x=6))

a, b = p
print(a, b)

print(Point._fields)
print(Point.__name__)
print(p._asdict())
print(p._replace(y=9))
print(Point._make([3, 4]))

Named = namedtuple("Named", "first second")
print(Named("hi", "there"))

P3 = namedtuple("P3", "a b c", defaults=[10, 20])
print(P3(1))
print(P3(1, 2))
print(P3._field_defaults)

R = namedtuple("R", ["abc", "def", "abc", "x"], rename=True)
print(R._fields)


def show(fn):
    try:
        return fn()
    except Exception as e:
        return type(e).__name__ + " " + str(e)


print(show(lambda: p._replace(z=9)))
print(show(lambda: Point._make([1])))
print(show(lambda: namedtuple("P", "a b", defaults=[1, 2, 3])))
print(show(lambda: namedtuple("P", "a 1b")))
print(show(lambda: namedtuple("P", "a def")))
print(show(lambda: namedtuple("P", "a a")))
print(show(lambda: namedtuple("P", "a _b")))
print(show(lambda: namedtuple("class", "a b")))
