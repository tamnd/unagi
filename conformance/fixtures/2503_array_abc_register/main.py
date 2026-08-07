import array
import collections.abc as abc

# The array module registers array.array as a collections.abc.MutableSequence
# at import, the way CPython's C module does, so an array answers the whole
# MutableSequence chain through the virtual-subclass registration.
a = array.array("i", [1, 2, 3])

print("== an array is a MutableSequence and everything it implies ==")
for name in [
    "MutableSequence",
    "Sequence",
    "Reversible",
    "Collection",
    "Iterable",
    "Container",
    "Sized",
]:
    print(name, isinstance(a, getattr(abc, name)))

print("== but not a mapping or a set ==")
print("MutableMapping", isinstance(a, abc.MutableMapping))
print("Mapping", isinstance(a, abc.Mapping))
print("MutableSet", isinstance(a, abc.MutableSet))
print("Set", isinstance(a, abc.Set))

print("== the registration is on the type, so issubclass agrees ==")
print("MutableSequence", issubclass(array.array, abc.MutableSequence))
print("Sequence", issubclass(array.array, abc.Sequence))
print("MutableMapping", issubclass(array.array, abc.MutableMapping))

print("== an empty array of another typecode registers the same ==")
print("float array", isinstance(array.array("d"), abc.MutableSequence))
print("byte array", isinstance(array.array("b", b"\x01\x02"), abc.Sequence))

print("== the sequence surface the ABC promises still works ==")
a.append(4)
a.insert(0, 0)
print("list", a.tolist())
print("index", a.index(3))
print("count", a.count(2))
print("reversed", list(reversed(a)))
del a[0]
print("after del", a.tolist())
