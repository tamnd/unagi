# A class reads its dunders off its own MRO before a metaclass's contribution.
# Every class's MRO ends in object, so C.__new__ is object.__new__, the plain
# allocator, even when the metaclass defines its own __new__ (type.__new__ and
# ABCMeta.__new__ both take name/bases/namespace). email.policy leans on this:
# Policy.clone does self.__class__.__new__(self.__class__) to copy an instance,
# and Policy is built through an ABCMeta metaclass.
from abc import ABCMeta, abstractmethod


class Policy(metaclass=ABCMeta):
    def __init__(self, tag):
        self.tag = tag

    def clone(self):
        other = self.__class__.__new__(self.__class__)
        other.tag = self.tag
        return other


p = Policy("a")
q = p.clone()
print(type(q).__name__)
print(isinstance(q, Policy))
print(q.tag)

# C.__new__ is exactly object.__new__, not the metaclass constructor.
print(Policy.__new__ is object.__new__)

# The object dunders inherit the same way through an ABCMeta class.
print(Policy.__repr__ is object.__repr__)
print(Policy.__init__ is not object.__init__)


# A concrete subclass of an abstract base still builds and runs its override.
class Shape(metaclass=ABCMeta):
    @abstractmethod
    def area(self):
        ...


class Square(Shape):
    def __init__(self, side):
        self.side = side

    def area(self):
        return self.side * self.side


print(Square(3).area())


# A genuine metaclass attribute the class MRO does not carry still resolves
# through the metaclass, so the fallback the fix preserves stays reachable.
class Meta(type):
    def tag(cls):
        return "meta:" + cls.__name__


class Widget(metaclass=Meta):
    pass


print(Widget.tag())
