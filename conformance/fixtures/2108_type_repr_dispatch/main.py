# A builtin type object exposes __repr__ the way CPython does, so pprint can key
# its dispatch table on it: _dispatch[dict.__repr__] = handler at registration
# and _dispatch.get(type(object).__repr__) at print time. That needs the
# attribute to resolve, to have stable identity, to be distinct per type, and to
# be callable as an unbound method that reprs the value.
import collections
import types

# Stable identity: the same read returns the same object both times.
print(dict.__repr__ is dict.__repr__)
print(list.__repr__ is list.__repr__)

# Distinct per type: each type carries its own repr wrapper.
print(dict.__repr__ is not list.__repr__)
print(set.__repr__ is not frozenset.__repr__)
print(bytes.__repr__ is not bytearray.__repr__)

# Callable as an unbound method, reprs the underlying value.
print(dict.__repr__({"a": 1}))
print(list.__repr__([1, 2, 3]))
print(tuple.__repr__((1, 2)))
print(set.__repr__({7}))
print(bytes.__repr__(b"hi"))

# Constructor-less type objects expose it too (mappingproxy, SimpleNamespace).
print(types.MappingProxyType.__repr__ is types.MappingProxyType.__repr__)
print(types.SimpleNamespace.__repr__ is not types.MappingProxyType.__repr__)

# The native collections constructors expose it as well, each distinct.
print(collections.deque.__repr__ is not collections.defaultdict.__repr__)
print(collections.Counter.__repr__ is not collections.ChainMap.__repr__)
print(collections.OrderedDict.__repr__ is collections.OrderedDict.__repr__)

# The full dispatch pattern: distinct keys land on distinct handlers, so a
# lookup by a type's __repr__ recovers the value registered for it.
dispatch = {}
dispatch[dict.__repr__] = "dict"
dispatch[list.__repr__] = "list"
dispatch[collections.Counter.__repr__] = "counter"
dispatch[collections.ChainMap.__repr__] = "chainmap"
dispatch[types.SimpleNamespace.__repr__] = "namespace"
print(len(dispatch))
print(dispatch.get(dict.__repr__))
print(dispatch.get(collections.Counter.__repr__))
print(dispatch.get(str.__repr__, "missing"))
