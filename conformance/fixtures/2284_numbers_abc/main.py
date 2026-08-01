# The numbers module is the numeric tower of ABCs (Number, Complex, Real,
# Rational, Integral) that the builtin and stdlib number types register into,
# and abc is the machinery behind it: ABCMeta, abstractmethod, register, and the
# __subclasshook__ structural test. This exercises both together.
import numbers
from fractions import Fraction
from decimal import Decimal

# The tower places each builtin type at its rung: int is Integral, float is Real
# but not Integral, complex is Complex but not Real, and every rung is a Number.
print("int:", isinstance(5, numbers.Integral), isinstance(5, numbers.Rational),
      isinstance(5, numbers.Real), isinstance(5, numbers.Number))
print("float:", isinstance(2.5, numbers.Integral), isinstance(2.5, numbers.Real),
      isinstance(2.5, numbers.Complex))
print("complex:", isinstance(1 + 2j, numbers.Real), isinstance(1 + 2j, numbers.Complex))
print("bool:", isinstance(True, numbers.Integral), issubclass(bool, numbers.Integral))
print("fraction:", isinstance(Fraction(3, 4), numbers.Rational),
      isinstance(Fraction(3, 4), numbers.Integral))
print("decimal:", isinstance(Decimal("1.5"), numbers.Number),
      isinstance(Decimal("1.5"), numbers.Real))

# issubclass walks the registered virtual subclasses too.
print("subclass:", issubclass(int, numbers.Real), issubclass(float, numbers.Complex),
      issubclass(complex, numbers.Integral))

import abc

# abstractmethod blocks instantiation until every abstract name is overridden.
class Shape(abc.ABC):
    @abc.abstractmethod
    def area(self):
        ...
    @property
    @abc.abstractmethod
    def name(self):
        ...

try:
    Shape()
except TypeError as e:
    print("abstract blocks:", "abstract" in str(e))

class Square(Shape):
    def __init__(self, side):
        self.side = side
    def area(self):
        return self.side * self.side
    @property
    def name(self):
        return "square"

sq = Square(4)
print("concrete:", sq.area(), sq.name)
print("isinstance:", isinstance(sq, Shape), issubclass(Square, Shape))

# A subclass that leaves an abstract method unimplemented is still abstract.
class Circle(Shape):
    def area(self):
        return 3
try:
    Circle()
except TypeError:
    print("partial still abstract")

# register makes an unrelated class a virtual subclass, no inheritance needed.
class Drawable(abc.ABC):
    @abc.abstractmethod
    def draw(self):
        ...

@Drawable.register
class Sprite:
    def draw(self):
        return "sprite"

print("registered:", issubclass(Sprite, Drawable), isinstance(Sprite(), Drawable))
# The registered class is not forced to be non-abstract, register is a promise.
print("register keeps type:", Sprite().draw())

# __subclasshook__ makes a structural ABC: any class defining the method counts,
# even without inheriting from the ABC.
class Sized(abc.ABC):
    @classmethod
    def __subclasshook__(cls, C):
        if cls is Sized:
            if any("__len__" in B.__dict__ for B in C.__mro__):
                return True
        return NotImplemented

class Roster:
    def __len__(self):
        return 2

class Tally:
    def count(self):
        return 0

print("structural:", issubclass(Roster, Sized), issubclass(Tally, Sized),
      isinstance(Roster(), Sized))

# ABCMeta directly, without the ABC helper base.
class Base(metaclass=abc.ABCMeta):
    @abc.abstractmethod
    def go(self):
        ...
class Impl(Base):
    def go(self):
        return "go"
print("abcmeta:", Impl().go(), isinstance(Impl(), Base))
