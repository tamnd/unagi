# object.__dir__ is readable and callable, returning the default attribute list
# an instance carries. A class that overrides __dir__ can call object.__dir__
# without recursing, the common `return object.__dir__(self) + [...]` shape.


class C:
    def __init__(self):
        self.x = 1

    def method(self):
        pass


c = C()
d = object.__dir__(c)
print(type(d).__name__)
print("__class__" in d, "__dir__" in d, "__eq__" in d)
print("x" in d, "method" in d)
# The bound form off the instance takes no extra argument.
print(c.__dir__() == d)


class D:
    def __dir__(self):
        return object.__dir__(self) + ["extra"]


dd = D()
print("extra" in dir(dd))
print("extra" in dd.__dir__())
print("__class__" in dd.__dir__())

# object.__dir__ is inherited, so every class exposes it.
print(hasattr(object, "__dir__"), hasattr(C, "__dir__"))
print(C.__dir__ is object.__dir__)
