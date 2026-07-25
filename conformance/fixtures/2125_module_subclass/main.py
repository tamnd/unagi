# A class may name types.ModuleType among its bases, the way importlib.util
# defines `class _LazyModule(types.ModuleType)` to swap a module's __class__ for
# lazy loading. The base is not an ordinary class, so it never joins the MRO;
# the subclass takes the module layout and inherits module.__init__.
import types


class M(types.ModuleType):
    pass


class Lazy(types.ModuleType):
    def greet(self):
        return "hi " + self.__name__


# The subclass really derives from ModuleType, and its instances are modules.
print(issubclass(M, types.ModuleType))
m = M("foo")
print(isinstance(m, types.ModuleType))
print(type(m) is M)

# module.__init__(name, doc=None) seeds __name__ and __doc__ from the arguments,
# positionally or by keyword; __doc__ defaults to None.
print(m.__name__, m.__doc__)
print(M("bar", "the docs").__doc__)
print(M(name="baz").__name__)

# An instance takes ordinary attributes, and a method the subclass adds runs
# against the seeded module state.
m.value = 41
print(m.value)
print(Lazy("world").greet())

# The bad-argument shapes match module.__init__.
try:
    M()
except TypeError:
    print("missing name")
try:
    M(5)
except TypeError:
    print("name not str")

# The real target: importlib.util defines _LazyModule this way, so it now
# imports and exposes LazyLoader.
import importlib.util

print(hasattr(importlib.util, "LazyLoader"))
