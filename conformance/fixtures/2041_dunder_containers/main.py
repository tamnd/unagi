# Builtin containers expose their protocol special methods as bound attribute
# reads and as direct calls: len(o), item in o and o[key] each have a __len__,
# __contains__ or __getitem__ that reads back as a callable, the way CPython's
# method-wrapper descriptors do. keyword.py leans on exactly this shape, binding
# iskeyword = frozenset(kwlist).__contains__ at import.
import keyword

# The module that motivates the feature: keyword builds its lookups from a bound
# frozenset.__contains__, so importing it at all proves the read path works.
print("iskeyword_for", keyword.iskeyword("for"))
print("iskeyword_xyz", keyword.iskeyword("xyz"))
print("issoftkeyword_match", keyword.issoftkeyword("match"))
print("issoftkeyword_for", keyword.issoftkeyword("for"))

# The same bound read on a frozenset of our own.
member = frozenset(["a", "b", "c"]).__contains__
print("bound_member", member("b"), member("z"))

# Mutable sequence: size, membership, index, assignment, deletion.
lst = [10, 20, 30]
print("list_len", lst.__len__())
print("list_contains", lst.__contains__(20), lst.__contains__(99))
print("list_getitem", lst.__getitem__(1))
lst.__setitem__(0, 11)
lst.__delitem__(2)
print("list_after", lst)

# Mapping: the subscript surface keyed by hash.
d = {"x": 1, "y": 2}
print("dict_len", d.__len__())
print("dict_contains", d.__contains__("x"), d.__contains__("z"))
print("dict_getitem", d.__getitem__("y"))
d.__setitem__("z", 3)
d.__delitem__("x")
print("dict_after", sorted(d.items()))

# Immutable sequences: read-only size, membership, index.
print("tuple", (1, 2, 3).__len__(), (1, 2, 3).__getitem__(2), (1, 2, 3).__contains__(2))
print("str", "abc".__len__(), "abc".__getitem__(0), "abc".__contains__("b"))
print("bytes", b"xy".__len__(), b"xy".__getitem__(1), b"xy".__contains__(120))
print("range", range(10).__len__(), range(10).__getitem__(4), range(10).__contains__(4))

# Sets carry size and membership but no subscript.
s = {1, 2, 3}
print("set", s.__len__(), s.__contains__(2), s.__contains__(9))
fs = frozenset([4, 5])
print("frozenset", fs.__len__(), fs.__contains__(5))

# A bound read is a first-class callable, stashed and used later, the exact
# shape keyword.iskeyword takes.
has_two = [1, 2, 3].__contains__
print("stashed", has_two(2), has_two(5))

# Reading a dunder a type does not expose is still an AttributeError: a set has
# no ordering to index, so set.__getitem__ does not exist.
try:
    {1, 2}.__getitem__(0)
except AttributeError:
    print("set_no_getitem", "AttributeError")

print("done")
