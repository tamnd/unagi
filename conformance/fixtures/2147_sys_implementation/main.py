import sys
import types

# sys.version_info is a struct sequence: it reprs with named fields, answers
# .major/.minor/.micro/.releaselevel/.serial, and still indexes and compares as
# the plain 5-tuple a version gate reads.
vi = sys.version_info
print(vi)
print(vi.major, vi.minor, vi.micro, vi.releaselevel, vi.serial)
print(vi[0], vi[1], vi[2])
print(vi[:3])
print(len(vi))
print(vi >= (3, 0), vi >= (3, 14), vi >= (4, 0))
print(tuple(vi))
print(vi == (3, 14, 6, "final", 0))

# types.SimpleNamespace is constructible: keyword arguments become attributes in
# first-assignment order, the repr is namespace(...), attributes read, mutate,
# extend and delete, and __dict__ is the live mapping.
ns = types.SimpleNamespace(a=1, b="x")
print(ns)
print(ns.a, ns.b)
ns.c = [1, 2]
ns.a = 99
print(ns)
del ns.b
print(ns)
print(ns.__dict__)
print(types.SimpleNamespace())
print(type(ns) is types.SimpleNamespace)
print(type(ns).__name__)

# the single positional form seeds from a mapping, then keywords override.
print(types.SimpleNamespace({"p": 1, "q": 2}, q=3))

try:
    ns.missing
except AttributeError as e:
    print("attr", e)
try:
    del ns.missing
except AttributeError as e:
    print("del", e)

# sys.implementation is a SimpleNamespace naming the interpreter. Its name,
# cache_tag, version and hexversion are the pinned CPython values; _multiarch is
# host-specific and supports_isolated_interpreters is a bool, so only their types
# are checked here.
im = sys.implementation
print(im.name)
print(im.cache_tag)
print(im.hexversion)
print(im.version)
print(im.version.major, im.version.releaselevel)
print(type(im._multiarch).__name__)
print(isinstance(im.supports_isolated_interpreters, bool))
print(im.version == sys.version_info)
