# collections.abc is the _collections_abc module, aliased the way the vendored
# collections package does it, so the dotted submodule imports and shares the
# one object.
import collections.abc
import _collections_abc

print(collections.abc is _collections_abc)
print(collections.abc.Sequence.__name__)
print(collections.abc.Mapping.__name__)
print(issubclass(list, collections.abc.Sequence))
print(issubclass(dict, collections.abc.Mapping))
print(isinstance([], collections.abc.Sequence))
print(isinstance({}, collections.abc.Mapping))

from collections import abc
print(abc is collections.abc)
print(abc.MutableSequence.__name__)

class MySeq:
    pass

collections.abc.Sequence.register(MySeq)
print(issubclass(MySeq, collections.abc.Sequence))
print(issubclass(MySeq, collections.abc.Mapping))
