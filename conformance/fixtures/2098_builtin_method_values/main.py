# A builtin container method reads back as a first-class value, so f = obj.method
# then f(...) matches obj.method(...). Covers str, bytes, bytearray, set,
# frozenset, dict with its Counter and OrderedDict kinds, and tuple.
import collections
import random

# str: read a method as a value, then call it.
upper = "hello".upper
print(upper())
split = "a,b,c".split
print(split(","))
join = "-".join
print(join(["x", "y", "z"]))
zfill = "7".zfill
print(zfill(4))
# The bound value equals a direct call.
print("Mixed".swapcase == "Mixed".swapcase or "Mixed".swapcase() == "mIXED")

# bytes: immutable, the read binds the same object.
lower = b"AbC".lower
print(lower())
hexit = b"\xde\xad".hex
print(hexit())
bfind = b"abcabc".find
print(bfind(b"c"))

# bytearray: a bound mutator changes the original array.
ba = bytearray(b"go")
append = ba.append
append(33)
print(ba)
extend = ba.extend
extend(b"!!")
print(ba)

# set: a bound add mutates the set, a bound union builds a new one.
s = {1, 2, 3}
add = s.add
add(4)
print(sorted(s))
discard = s.discard
discard(2)
print(sorted(s))
union = s.union
print(sorted(union({10, 11})))
issub = s.issubset
print(issub({1, 3, 4, 10}))

# frozenset: only the non-mutating surface reads back.
fz = frozenset({5, 6})
funion = fz.union
print(sorted(funion({7})))
finter = fz.intersection
print(sorted(finter({6, 9})))

# dict: get, setdefault and items as values.
d = {"a": 1, "b": 2}
get = d.get
print(get("a"), get("z", -1))
setdefault = d.setdefault
setdefault("c", 3)
print(sorted(d.items()))
keys = d.keys
print(sorted(keys()))

# Counter and OrderedDict expose their extra methods as values too.
cnt = collections.Counter("aaabbc")
most_common = cnt.most_common
print(most_common(2))
elements = cnt.elements
print(sorted(elements()))
od = collections.OrderedDict([("x", 1), ("y", 2), ("z", 3)])
move_to_end = od.move_to_end
move_to_end("x")
print(list(od))

# tuple: count and index read back.
t = (1, 2, 2, 3, 2)
count = t.count
print(count(2))
index = t.index
print(index(3))

# An unknown attribute is still the plain AttributeError, hasattr agrees.
print(hasattr("x", "upper"), hasattr("x", "nope"))
print(hasattr({1}, "add"), hasattr({1}, "append"))
print(hasattr(frozenset(), "union"), hasattr(frozenset(), "add"))
try:
    _ = "x".casefold_missing
except AttributeError:
    print("str unknown -> AttributeError")

# The large-population sample path selects through set.add read as a value, which
# only runs now that the read binds. Seeded, so the pick is fixed.
random.seed(5)
print(sorted(random.sample(range(1000), 3)))

# The protocol methods still resolve past the new method gate.
print("ab".__len__(), {1, 2}.__len__(), (1, 2, 3).__len__())
print("ab".__contains__("a"), frozenset({1}).__contains__(1))
