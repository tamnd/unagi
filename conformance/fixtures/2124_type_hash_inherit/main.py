# A type answers __hash__ off the type object, the way dataclasses reads
# `f.default.__class__.__hash__ is None` to reject a mutable default. A hashable
# type reads back object's hash slot, an unhashable container reads back None.
class A:
    pass


class WithEq:
    def __eq__(self, other):
        return True


class WithHash:
    def __hash__(self):
        return 7


def fn():
    pass


# The builtin constructors: the scalar and immutable types are hashable, the
# mutable containers carry __hash__ = None.
print(int.__hash__ is None, str.__hash__ is None, float.__hash__ is None)
print(list.__hash__ is None, dict.__hash__ is None, set.__hash__ is None, bytearray.__hash__ is None)

# A user class inherits object.__hash__ unless it defines __eq__ without its own
# __hash__, which nulls it; a class defining __hash__ keeps it.
print(A.__hash__ is None, WithEq.__hash__ is None, WithHash.__hash__ is None)

# A constructor-less builtin type, the function type among them, is hashable.
print(type(fn).__hash__ is None)

# The inherited slot is one shared object: a class that inherits it reads back
# the same wrapper object as object does, and each read is stable.
print(A.__hash__ is object.__hash__)
print(int.__hash__ is int.__hash__)

# The slot is callable and agrees with hash(): int.__hash__(x) equals hash(x),
# and the inherited object slot hashes an instance by identity.
a = A()
print(A.__hash__(a) == hash(a))
print(int.__hash__(5) == hash(5))
print(str.__hash__("abc") == hash("abc"))
print(type(fn).__hash__(fn) == hash(fn))
