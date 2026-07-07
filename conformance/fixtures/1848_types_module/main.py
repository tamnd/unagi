# The types module compiles from its bundled source. It builds on the _types
# builtin for the concrete type objects and adds a real MappingProxyType.
import types

# MappingProxyType wraps a dict as a read-only view.
d = {"a": 1, "b": 2}
mp = types.MappingProxyType(d)
print(type(mp).__name__)
print(mp["a"], "b" in mp, len(mp))
print(sorted(mp.keys()), sorted(mp.items()))
print(mp.get("a"), mp.get("z", -1))

# The view tracks the underlying dict.
d["c"] = 3
print(mp["c"], len(mp))

# Writes and deletes are refused with the read-only messages.
try:
    mp["x"] = 9
except TypeError as e:
    print(e)
try:
    del mp["a"]
except TypeError as e:
    print(e)

# The concrete type objects identify runtime objects.
def fn():
    yield 1


print(type(fn) is types.FunctionType)
print(type(fn()) is types.GeneratorType)
print(type(len) is types.BuiltinFunctionType)
print(types.NoneType is type(None))
print(type(mp) is types.MappingProxyType)

# The type objects repr as their class, and the aliases share one object.
print(types.FunctionType)
print(types.GeneratorType)
print(types.MappingProxyType)
print(types.FunctionType is types.LambdaType)

# DynamicClassAttribute is defined in the module source and is subclassable.
class D(types.DynamicClassAttribute):
    pass


print(D.__name__, issubclass(D, types.DynamicClassAttribute))

# Constructing a constructor-less type object raises.
try:
    types.GeneratorType()
except TypeError as e:
    print(e)

# A non-mapping to MappingProxyType is rejected.
try:
    types.MappingProxyType([1, 2])
except TypeError as e:
    print(e)
