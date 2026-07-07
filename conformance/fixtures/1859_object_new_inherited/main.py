# object.__new__ is a single canonical allocator that classes inherit and
# NoneType reports too, the shape enum's _find_new_ builds a set out of.


class Plain:
    pass


class Custom:
    def __new__(cls, *args, **kwargs):
        return super().__new__(cls)


# The allocator is a builtin, and reads back as the same object every time.
print(type(object.__new__).__name__)
print(object.__new__ is object.__new__)

# A class with no __new__ of its own inherits object.__new__.
print(Plain.__new__ is object.__new__)
print(getattr(object, "__new__", None) is object.__new__)

# A class that defines __new__ shadows the inherited one.
print(Custom.__new__ is not object.__new__)

# Calling the allocator builds a bare instance of the class handed in.
made = object.__new__(Plain)
print(isinstance(made, Plain))
print(type(made) is Plain)

# NoneType inherits it too, so None.__new__ resolves without error and the
# set literal enum writes over these names is constructible.
print(hasattr(None, "__new__"))
names = {None, object.__new__, Custom.__new__}
print(len(names))
print(object.__new__ in names)
