# A builtin type inherits object's dunders as readable attributes, the tail of
# every builtin type's MRO, matching what a user-defined class already gets.
INHERITED = [
    "__repr__", "__str__", "__format__", "__init__", "__reduce__",
    "__reduce_ex__", "__subclasshook__", "__eq__", "__ne__", "__dir__",
    "__getstate__", "__lt__", "__le__", "__gt__", "__ge__", "__hash__",
]
TYPES = [int, str, float, bool, list, dict, tuple, set, frozenset, bytes,
         bytearray, complex]

# Every inherited dunder resolves on every builtin type, and is callable
# (or None for the unhashable containers' __hash__).
for t in TYPES:
    flags = []
    for d in INHERITED:
        v = getattr(t, d)
        flags.append("1" if (callable(v) or v is None) else "0")
    print(t.__name__, "".join(flags))

# hasattr agrees with a user-defined class for the same names.
class C:
    pass
print("parity", all(hasattr(int, d) == hasattr(C, d) for d in INHERITED))

# The object-inherited slots that a builtin type does not override behave as
# object's: __init__ is a no-op returning None and __subclasshook__ declines.
print("init", int.__init__(5) is None)
print("hook", tuple.__subclasshook__(tuple) is NotImplemented)

# The unhashable containers read back __hash__ is None; the hashable ones a
# callable.
print("hash_none", list.__hash__ is None, dict.__hash__ is None)
print("hash_call", callable(int.__hash__), callable(str.__hash__))
