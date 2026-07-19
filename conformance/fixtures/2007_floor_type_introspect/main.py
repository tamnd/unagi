# A constructor-less builtin type answers the type introspection attributes.
# _collections_abc registers coroutine, generator, and the iterator and view
# types as virtual subclasses of its ABCs, and _py_abc._check_methods opens with
# `mro = C.__mro__` then probes `method in B.__dict__` for each B, so every such
# type has to carry __mro__, __bases__, __base__, and a __dict__. unagi gives
# them the plain (T, object) chain rooted at object; the type's own methods are
# not enumerated in __dict__ yet, so the membership probe misses and the ABC
# falls back to its registry. The type names differ from CPython (unagi shares
# one iterator type), so this checks the shape, not the names.
def gen():
    yield 1


async def coro():
    return 1


c = coro()
singletons = [type(gen()), type(c), type(iter([])), type(iter(())), type(iter("")),
              type(iter({}.keys())), type(iter(range(1)))]
for t in singletons:
    mro = t.__mro__
    print(len(mro), mro[0] is t, mro[-1] is object,
          t.__bases__ == (object,), t.__base__ is object, "missing" in t.__dict__)
c.close()

# The __dict__ is a read-only mappingproxy: a write through it is a TypeError.
proxy = type(gen()).__dict__
try:
    proxy["x"] = 1
except TypeError as e:
    print("readonly:", "does not support item assignment" in str(e))
