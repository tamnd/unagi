# An import statement inside a class body, the shape email.message.Message uses
# when it writes `from email.iterators import walk` to install a function as a
# method. The import runs as the body executes, so the bound name lands in the
# class namespace like any other assignment, CPython's STORE_NAME, and reads
# back as a class attribute.

import math


class Calc:
    from math import gcd, lcm

    import math as m

    base = 10


# The from-imported names and the aliased module are class attributes now.
print(Calc.gcd(12, 8))
print(Calc.lcm(4, 6))
print(Calc.m.floor(3.7))
print(Calc.base)
print(Calc.gcd is math.gcd)
print(hasattr(Calc, "gcd"), hasattr(Calc, "lcm"), hasattr(Calc, "m"))


# A method reads an imported name off the class through self, since it lives in
# the same namespace the method's own definition did.
class Ops:
    from math import gcd

    def reduce(self, a, b):
        return self.gcd(a, b)


print(Ops().reduce(24, 36))


# An import guarded by a branch in the class body binds only when its branch
# runs, so the store routes through the class builder inside the control flow,
# not into a stray variable.
USE_FRACTIONS = True


class Guarded:
    if USE_FRACTIONS:
        from math import factorial as fact
    else:
        from math import isqrt as fact


print(Guarded.fact(5))
print(hasattr(Guarded, "fact"))
