# A mappingproxy has no __str__ of its own, so str() delegates to the wrapped
# mapping and prints {...}; only repr wraps it as mappingproxy({...}). That is
# why print(cls.__members__) on an enum shows the plain dict.
from types import MappingProxyType

mp = MappingProxyType({"a": 1, "b": 2})
print(mp)
print(str(mp))
print(repr(mp))
print(f"{mp}")
print([mp])
print("{}".format(mp))

empty = MappingProxyType({})
print(empty)
print(repr(empty))

# A nested proxy value still reprs with the wrapper inside a container.
outer = MappingProxyType({"inner": mp})
print(outer)
print(repr(outer))
