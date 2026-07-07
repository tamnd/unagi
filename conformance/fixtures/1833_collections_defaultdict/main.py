# collections.defaultdict, a dict subclass provided in Go behind the collections
# import. This exercises the factory fill on a missing key, the None factory that
# falls back to KeyError, seeding from a mapping, the default_factory attribute
# read and write, copy keeping the factory, and equality with a plain dict.
import collections
from collections import defaultdict

counts = defaultdict(int)
for ch in "mississippi":
    counts[ch] += 1
print(dict(counts))
print(counts)

groups = defaultdict(list)
for name, size in [("a", 1), ("b", 2), ("a", 3), ("c", 4), ("b", 5)]:
    groups[name].append(size)
print(dict(groups))

seeded = defaultdict(int, {"x": 5})
print(seeded["x"], seeded["y"])
print(seeded)

plain = defaultdict(int)
print(plain.default_factory)
plain["z"]
print(dict(plain))

d = defaultdict()
print(d.default_factory)


def missing_no_factory():
    dd = defaultdict()
    try:
        return dd["nope"]
    except KeyError as e:
        return str(e)


print(missing_no_factory())


def bad_factory():
    try:
        defaultdict(5)
    except TypeError as e:
        return str(e)


print(bad_factory())

late = defaultdict()
late.default_factory = list
late["k"].append(1)
print(late)

c = defaultdict(list, {"p": [1]})
c2 = c.copy()
c2["q"].append(2)
print(c, c2)

print(defaultdict(int, {"a": 1, "b": 2}) == {"a": 1, "b": 2})

kw = defaultdict(int, a=1, b=2)
print(dict(kw))
