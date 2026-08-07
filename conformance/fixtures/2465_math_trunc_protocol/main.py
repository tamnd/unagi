# math.trunc requires __trunc__ for anything that is not a genuine int or float:
# unlike floor and ceil it never coerces through __index__ or __float__, so a
# bare __index__ object raises rather than silently truncating. An int subclass
# and a float subclass truncate through the slot they inherit.
import math
from fractions import Fraction
from decimal import Decimal

def show(label, fn):
    try:
        print(label, "=>", repr(fn()))
    except Exception as e:
        print(label, "=>", type(e).__name__, ":", e)

class OnlyIndex:
    def __index__(self):
        return 5

class OnlyFloat:
    def __float__(self):
        return 2.7

class MyInt(int):
    pass

class MyFloat(float):
    pass

class OverrideTrunc(float):
    def __trunc__(self):
        return 999

show("trunc(3.9)", lambda: math.trunc(3.9))
show("trunc(-3.9)", lambda: math.trunc(-3.9))
show("trunc(0.0)", lambda: math.trunc(0.0))
show("trunc(7)", lambda: math.trunc(7))
show("trunc(True)", lambda: math.trunc(True))
show("trunc(10**30)", lambda: math.trunc(10**30))
show("trunc(1e30)", lambda: math.trunc(1e30))
show("trunc(inf)", lambda: math.trunc(math.inf))
show("trunc(nan)", lambda: math.trunc(math.nan))
show("trunc(Fraction(7,2))", lambda: math.trunc(Fraction(7, 2)))
show("trunc(Fraction(-7,2))", lambda: math.trunc(Fraction(-7, 2)))
show("trunc(Decimal('3.99'))", lambda: math.trunc(Decimal('3.99')))
show("trunc(MyInt(9))", lambda: math.trunc(MyInt(9)))
show("trunc(MyFloat(3.9))", lambda: math.trunc(MyFloat(3.9)))
show("trunc(OverrideTrunc(3.9))", lambda: math.trunc(OverrideTrunc(3.9)))
show("trunc(OnlyIndex())", lambda: math.trunc(OnlyIndex()))
show("trunc(OnlyFloat())", lambda: math.trunc(OnlyFloat()))
show("trunc('x')", lambda: math.trunc('x'))
show("trunc(None)", lambda: math.trunc(None))
