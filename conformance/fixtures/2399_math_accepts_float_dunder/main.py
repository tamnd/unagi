# math functions coerce their argument through CPython's PyFloat_AsDouble, which
# honours __float__ then __index__, so a Fraction, a Decimal or any user object
# spelling one of those converts the way it does under float() rather than being
# rejected with "must be real number". The geometric_mean results chain log, exp
# and fsum, so they ride the Go-vs-libm last-ULP ceiling and are rounded.
import math
import statistics as st
from fractions import Fraction as F
from decimal import Decimal as D


def show(label, fn):
    try:
        print(label, repr(fn()))
    except Exception as e:
        print(label, type(e).__name__, str(e))


# Fraction through the single-argument float routines.
show("log", lambda: math.log(F(1, 2)))
show("sqrt", lambda: math.sqrt(F(1, 4)))
show("exp", lambda: math.exp(F(0, 1)))
show("sin", lambda: math.sin(F(0, 1)))
show("floor", lambda: math.floor(F(7, 2)))

# Fraction through the two and three argument routines.
show("atan2", lambda: math.atan2(F(1, 2), F(1, 3)))
show("copysign", lambda: math.copysign(F(-3, 2), F(1, 1)))
show("hypot", lambda: math.hypot(F(3, 1), F(4, 1)))
show("dist", lambda: math.dist([F(0), F(0)], [F(3), F(4)]))
show("fmod", lambda: math.fmod(F(7, 2), F(2, 1)))
show("pow", lambda: math.pow(F(2, 1), F(3, 1)))
show("logbase", lambda: math.log(F(8, 1), F(2, 1)))
show("isclose", lambda: math.isclose(F(1, 3), 0.3333333333333333))

# Decimal converts through __float__ the same way.
show("dec-sqrt", lambda: math.sqrt(D("0.25")))
show("dec-log", lambda: math.log(D("2.5")))

# statistics.geometric_mean over Fractions and Decimals leans on math.log, which
# now accepts them (rounded for the transcendental last-ULP ceiling).
show("gmean-F", lambda: round(st.geometric_mean([F(1, 2), F(3, 2)]), 9))
show("gmean-D", lambda: round(st.geometric_mean([D("1.5"), D("2.5")]), 9))


# A user object with __float__ or __index__ converts; the return-type and
# not-a-number error messages match CPython.
class HasFloat:
    def __float__(self):
        return 2.0


class HasIndex:
    def __index__(self):
        return 3


class BadFloat:
    def __float__(self):
        return "x"


show("user-float", lambda: math.log(HasFloat()))
show("user-index", lambda: math.sqrt(HasIndex()))
show("bad-float", lambda: math.sqrt(BadFloat()))
show("str", lambda: math.sqrt("x"))
show("list", lambda: math.sqrt([1]))
