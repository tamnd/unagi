# type.__dict__ carries the getset descriptors inspect binds to read a class's
# linearization and namespace without a metaclass __getattribute__ in the way:
# _static_getmro = type.__dict__['__mro__'].__get__ and
# _get_dunder_dict_of_class = type.__dict__['__dict__'].__get__. Both were
# missing, so `import inspect` (and everything that imports it) stopped at a
# KeyError. This adds them and imports inspect and dataclasses end to end.

# The descriptors are present and are getset descriptors.
mro_desc = type.__dict__['__mro__']
dict_desc = type.__dict__['__dict__']
print(type(mro_desc).__name__, type(dict_desc).__name__)
print(repr(mro_desc))
print(repr(dict_desc))

getmro = mro_desc.__get__
getdict = dict_desc.__get__


class A:
    pass


class B(A):
    pass


# __mro__ reads a user class's linearization and a builtin type's.
print(getmro(B))
print(getmro(int))

# __dict__ reads a class namespace as a mappingproxy.
d = getdict(B)
print(type(d).__name__)

T = type('T', (), {'attr': 1})
print('attr' in getdict(T))

# The payoff: inspect and dataclasses now import.
import inspect
import dataclasses

print(inspect.__name__, dataclasses.__name__)
print(inspect.isclass(B), inspect.isclass(int), inspect.isclass(getmro))
