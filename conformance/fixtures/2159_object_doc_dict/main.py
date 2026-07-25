# The object root carries a canonical __doc__ string (every other class with no
# docstring still reads None), and object.__dict__ is a populated mappingproxy
# that lists the dunders object actually provides rather than an empty view.


print(object.__doc__)


class WithDoc:
    "my doc"


class NoDoc:
    pass


# A user class reads its own docstring, or None when it declared none. The
# object root is the only type with the canonical string.
print(repr(WithDoc.__doc__), repr(NoDoc.__doc__))

# object.__dict__ is a mappingproxy that carries the object surface.
print(type(object.__dict__).__name__)
present = [
    "__init__", "__new__", "__eq__", "__ne__", "__lt__", "__le__",
    "__gt__", "__ge__", "__repr__", "__str__", "__format__", "__hash__",
    "__getattribute__", "__setattr__", "__delattr__", "__dir__",
    "__getstate__", "__reduce__", "__reduce_ex__", "__subclasshook__",
    "__doc__",
]
print(all(name in object.__dict__ for name in present))
print("not_a_real_name" in object.__dict__)
print(len(object.__dict__) > 10)

# The docstring reads back the same off __dict__ and the attribute.
print(object.__dict__["__doc__"] == object.__doc__)
