# collections.Counter, a dict subclass provided in Go behind the collections
# import. This exercises counting an iterable, seeding from a mapping and
# keywords, the zero read on a missing element, most_common, elements, the
# arithmetic operators keeping only positive counts, unary plus and minus,
# update and subtract, total, equality with a plain dict, and the dict-union
# fallback when the right operand is not a Counter.
import collections
from collections import Counter

c = Counter("mississippi")
print(c)
print(c["s"], c["missing"])
print(len(c))
print(c.most_common(2))
print(sorted(c.elements()))

print(Counter({"x": 5, "y": 2}))
print(Counter("ab", a=1))
print(Counter())

print(Counter("abbccc") + Counter("bcc"))
print(Counter(a=3, b=1) - Counter(a=1, b=2))
print(Counter(a=3, b=1) & Counter(a=1, b=2))
print(Counter(a=3, b=1) | Counter(a=1, b=2))
print(+Counter(a=1, b=-1))
print(-Counter(a=1, b=-1))

d = Counter("aab")
d.subtract("ab")
print(d)
d.update(["a", "x"])
print(d)

print(Counter({"x": 5, "y": 2}).total())
print(Counter(a=1) == {"a": 1})
print(dict(Counter("aab")))


def add_dict():
    try:
        return Counter(a=1) + {"a": 2}
    except TypeError as e:
        return str(e)


print(add_dict())

u = Counter(a=1) | {"a": 2}
print(u, type(u).__name__)
