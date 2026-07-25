# functools.partial is a real type, not just a callable factory. inspect.signature
# branches on isinstance(obj, functools.partial), so it has to answer type(),
# isinstance, issubclass and repr as a class the way CPython's C partial does.
import functools


def add(a, b, c=0):
    return a + b + c


p = functools.partial(add, 1, c=10)

# It calls through, freezing the leading positional and the keyword.
print(p(2))

# type(partial) is the metatype, and an instance reports the partial type.
print(type(functools.partial) is type)
print(type(p) is functools.partial)

# isinstance and issubclass treat it as a class.
print(isinstance(p, functools.partial))
print(isinstance(p, object))
print(isinstance(5, functools.partial))
print(issubclass(functools.partial, functools.partial))
print(issubclass(functools.partial, object))

# The frozen state reads back off the instance.
print(p.func is add, p.args, sorted(p.keywords.items()))

# The construction guards match CPython.
try:
    functools.partial()
except TypeError:
    print("noarg")
try:
    functools.partial(5)
except TypeError:
    print("notcallable")
