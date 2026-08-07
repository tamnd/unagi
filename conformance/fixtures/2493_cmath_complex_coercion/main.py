import cmath
from decimal import Decimal
from fractions import Fraction

def show(label, fn):
    try:
        print(label, "=>", repr(fn()))
    except Exception as e:
        print(label, "=>", type(e).__name__, ":", e)

class MyComplex:
    def __init__(self, z): self.z = z
    def __complex__(self): return complex(self.z)
class MyFloat:
    def __float__(self): return 2.0
class MyIndex:
    def __index__(self): return 3
class BadComplex:
    def __complex__(self): return "nope"
class RaiseComplex:
    def __complex__(self): raise ValueError("boom")

# __complex__ wins and its result must be a complex.
show("sqrt(MyComplex(4))", lambda: cmath.sqrt(MyComplex(4)))
show("phase(MyComplex(1j))", lambda: cmath.phase(MyComplex(1j)))
show("sqrt(BadComplex())", lambda: cmath.sqrt(BadComplex()))
show("sqrt(RaiseComplex())", lambda: cmath.sqrt(RaiseComplex()))
# __float__ and __index__ stand in for a real value.
show("sqrt(MyFloat())", lambda: cmath.sqrt(MyFloat()))
show("sqrt(MyIndex())", lambda: cmath.sqrt(MyIndex()))
# A Decimal or Fraction converts through its __float__.
show("sqrt(Decimal(4))", lambda: cmath.sqrt(Decimal(4)))
show("isclose(D,D)", lambda: cmath.isclose(Decimal("1.0"), Decimal("1.0")))
show("isclose(F,F)", lambda: cmath.isclose(Fraction(1, 2), Fraction(1, 2)))
# A type with none of these keeps the real-number error, and a huge int overflows.
show("sqrt('x')", lambda: cmath.sqrt("x"))
show("sqrt(None)", lambda: cmath.sqrt(None))
show("sqrt(10**1000)", lambda: cmath.sqrt(10**1000))
# Plain numbers are unchanged.
show("sqrt(2)", lambda: cmath.sqrt(2))
show("sqrt(True)", lambda: cmath.sqrt(True))
show("sqrt(-1+0j)", lambda: cmath.sqrt(-1 + 0j))
