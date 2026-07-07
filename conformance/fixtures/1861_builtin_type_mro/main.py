# The builtin type constructors double as type objects, so they answer the
# linearization attributes EnumType walks when it checks a mixin base.

# A type that derives straight from object reports a two-link chain.
print(int.__mro__)
print(int.__bases__)
print(int.__base__)

print(str.__mro__)
print(str.__bases__)
print(str.__base__)

# bool derives from int, so its chain has the extra link and its base is int.
print(bool.__mro__)
print(bool.__bases__)
print(bool.__base__)

# The tuple really holds the type objects, not their names.
print(int.__mro__[0] is int)
print(int.__mro__[-1] is object)
print(bool.__base__ is int)
print(bool.__mro__[1] is int)

# __qualname__ reads the bare type name.
print(int.__qualname__)
print(bool.__qualname__)
