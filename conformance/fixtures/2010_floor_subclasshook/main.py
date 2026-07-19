# object.__subclasshook__ is the default every class inherits: a classmethod
# returning NotImplemented so _py_abc.__subclasscheck__ can call
# cls.__subclasshook__(subclass) unconditionally and fall through to its registry
# when the class defines no structural test. Without it that call raised
# AttributeError and an ABCMeta class could not run isinstance.
class C:
    pass


# The default resolves on a class, on object, and on an instance, and always
# answers NotImplemented.
print(hasattr(C, "__subclasshook__"))
print(C.__subclasshook__(int) is NotImplemented)
print(object.__subclasshook__(list) is NotImplemented)
print(hasattr(object(), "__subclasshook__"))
print(C().__subclasshook__(str) is NotImplemented)


# A class that defines its own __subclasshook__ shadows the object default, so a
# structural check drives isinstance rather than the fallthrough.
class HasFoo:
    @classmethod
    def __subclasshook__(cls, other):
        return hasattr(other, "foo")


class WithFoo:
    foo = 1


print(HasFoo.__subclasshook__(WithFoo))
print(HasFoo.__subclasshook__(C))


# An ABCMeta class with no structural hook still runs isinstance through the
# object default and its registry.
import _collections_abc as abc


class Shape(metaclass=abc.ABCMeta):
    pass


class Square(Shape):
    pass


print(isinstance(Square(), Shape))
print(isinstance(C(), Shape))
