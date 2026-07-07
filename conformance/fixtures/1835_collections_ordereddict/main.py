# collections.OrderedDict, a dict subclass provided in Go behind the collections
# import. A plain dict already preserves order, so this exercises the order-aware
# extras: move_to_end at both ends, popitem selecting either end, order-sensitive
# equality against another OrderedDict but order-insensitive against a plain
# dict, the OrderedDict-preserving union, reversed iteration, and copy keeping
# the subclass.
import collections
from collections import OrderedDict

o = OrderedDict([("a", 1), ("b", 2), ("c", 3)])
print(o)

o.move_to_end("a")
print(o)
o.move_to_end("c", last=False)
print(o)

print(o.popitem())
print(o.popitem(last=False))
print(o)

print(OrderedDict(x=1, y=2))
print(OrderedDict({"p": 1, "q": 2}))

print(list(reversed(OrderedDict([("a", 1), ("b", 2), ("c", 3)]))))

print(OrderedDict([("a", 1), ("b", 2)]) == OrderedDict([("b", 2), ("a", 1)]))
print(OrderedDict([("a", 1), ("b", 2)]) == OrderedDict([("a", 1), ("b", 2)]))
print(OrderedDict([("a", 1), ("b", 2)]) == {"b": 2, "a": 1})

u = OrderedDict(a=1) | {"b": 2}
print(u, type(u).__name__)

c = OrderedDict([("a", 1), ("b", 2)])
c2 = c.copy()
c2["c"] = 3
print(type(c2).__name__, c, c2)

print(dict(OrderedDict([("a", 1)])))


def empty_popitem():
    try:
        OrderedDict().popitem()
    except KeyError as e:
        return str(e)


print(empty_popitem())


def missing_move():
    try:
        OrderedDict([("a", 1)]).move_to_end("z")
    except KeyError as e:
        return str(e)


print(missing_move())
