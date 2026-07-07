# object.__init__ is an inherited no-op initializer returning None, the shape an
# enum member falls back to when it defines a value but no __init__ of its own
# and creation calls enum_member.__init__(*args).


class C:
    pass


c = C()

# A plain instance inherits object's initializer, which returns None.
print(c.__init__() is None)

# A class defining no __init__ of its own reads back object's, one canonical
# object either way.
print(C.__init__ is object.__init__)
print(object.__init__ is object.__init__)


# A class with its own __new__ but no __init__ lets object.__init__ swallow the
# extra constructor arguments.
class D:
    def __new__(cls, *args):
        return object.__new__(cls)


d = D(1, 2, 3)
print(isinstance(d, D))
print(D.__init__ is object.__init__)
print(d.__init__(4, 5) is None)
