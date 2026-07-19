# _collections_abc.Hashable is a structural ABC: its __subclasshook__ asks
# whether __hash__ is present and truthy in the candidate type's __dict__, with
# no register call. A builtin type therefore has to carry __hash__ in its own
# __dict__ the way CPython does: a real callable for the hashable builtins, the
# None sentinel for the mutable containers that disable hashing, and nothing for
# bool (which inherits int's through the MRO). Without that entry every
# isinstance(x, Hashable) came back False and the collections ABCs diverged.
import _collections_abc as abc

# Hashable builtins report True; the mutable containers report False.
print(isinstance("x", abc.Hashable))
print(isinstance(5, abc.Hashable))
print(isinstance((1, 2), abc.Hashable))
print(isinstance(frozenset(), abc.Hashable))
print(isinstance(b"x", abc.Hashable))
print(isinstance([1], abc.Hashable))
print(isinstance({1: 2}, abc.Hashable))
print(isinstance({1}, abc.Hashable))
print(isinstance(bytearray(b"x"), abc.Hashable))

# bool has no __hash__ of its own but inherits int's, so it is Hashable too.
print(isinstance(True, abc.Hashable))

# The __dict__ entry is present for a hashable builtin, absent-as-None for a
# container, and it computes the value's own hash when called.
print("__hash__" in str.__dict__, "__hash__" in list.__dict__)
print(str.__dict__["__hash__"]("x") == hash("x"))
print(list.__dict__["__hash__"] is None)
