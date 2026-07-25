# object.__getstate__ is readable and callable, returning the default pickle
# state CPython 3.11+ computes: a populated __dict__ as is, __slots__ as a
# (dictstate, slotstate) pair, and None when the instance carries neither.
# object.__reduce__ resolves as an attribute (hasattr sees it), though the real
# copyreg-shaped reduction waits on copyreg.


class Empty:
    pass


class WithDict:
    def __init__(self):
        self.x = 1
        self.y = 2


class WithSlots:
    __slots__ = ("a", "b")

    def __init__(self):
        self.a = 5
        self.b = 6


# An instance with no state answers None.
print(Empty().__getstate__())
print(object().__getstate__())

# A populated __dict__ comes back as is, and it is the live dict object.
wd = WithDict()
print(wd.__getstate__())
print(wd.__getstate__() is wd.__dict__)

# __slots__ come back as (None, slotstate) with no instance dict.
print(WithSlots().__getstate__())

# The unbound form off the type object takes the instance explicitly.
print(object.__getstate__(wd))

# Every object inherits the slot, so it reads off the type and the instance.
print(hasattr(object, "__getstate__"), hasattr(wd, "__getstate__"))
print(WithDict.__getstate__ is object.__getstate__)

# __reduce__ resolves as an attribute the way CPython exposes it.
print(hasattr(object, "__reduce__"), hasattr(Empty(), "__reduce__"))
