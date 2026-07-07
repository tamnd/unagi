# object carries its default dunder methods, the ones EnumType.__new__ reads off
# object and off the enum class to decide which are still the object default.


class Plain:
    pass


class Custom:
    def __repr__(self):
        return "Custom!"

    def __format__(self, spec):
        return "fmt:" + spec


DUNDERS = ("__repr__", "__str__", "__format__", "__reduce_ex__")

# Each default reads back as one shared object, so a class that overrides none
# of them inherits object's, and getattr off object hands back that same object.
for name in DUNDERS:
    print(name, getattr(object, name) is getattr(object, name))
    print(name, getattr(Plain, name) is getattr(object, name))

# A class that defines its own shadows the inherited default.
print(Custom.__repr__ is not object.__repr__)
print(Custom.__format__ is not object.__format__)
print(Custom.__str__ is object.__str__)

# The defaults are callable and produce the object-root results.
p = Plain()
print(object.__repr__(p) == repr(p))
print(object.__str__(p) == repr(p))
print(object.__format__(p, "") == str(p))

# An instance reads each default bound to itself.
print(hasattr(p, "__repr__"))
print(p.__repr__() == repr(p))
print(p.__format__("") == str(p))

# A non-empty format spec on the object default is a TypeError.
try:
    object.__format__(p, "d")
except TypeError as e:
    print("TypeError", "unsupported format string" in str(e))
