# math gains cbrt, the real cube root, and exp2, two raised to a power. Both are
# libm transcendentals, so Go and CPython can differ by a last-bit ulp on a
# general argument. The fixture asserts only the points the two libraries agree
# on exactly: perfect cubes, exact powers of two, the signs and infinities, and
# exp2's overflow.

import math

# cbrt is exact on perfect cubes and keeps the sign, unlike sqrt it accepts
# negatives.
print(math.cbrt(8.0))
print(math.cbrt(27.0))
print(math.cbrt(1000.0))
print(math.cbrt(-8.0))
print(math.cbrt(0.125))
print(math.cbrt(0.0))
print(math.cbrt(-0.0))
print(math.cbrt(1.0))

# cbrt passes infinities and nan straight through.
print(math.cbrt(math.inf))
print(math.cbrt(-math.inf))
print(math.isnan(math.cbrt(math.nan)))

# exp2 is exact on integer exponents, the plain powers of two.
print(math.exp2(0.0))
print(math.exp2(1.0))
print(math.exp2(10.0))
print(math.exp2(-2.0))
print(math.exp2(-10.0))

# exp2 at the range edges and infinities.
print(math.exp2(-math.inf))
print(math.exp2(math.inf))
print(math.isnan(math.exp2(math.nan)))

# bool and int arguments convert to float.
print(math.cbrt(True))
print(math.exp2(3))


def show(label, fn):
    try:
        print(label, '->', fn())
    except (TypeError, ValueError, OverflowError) as ex:
        print(label, '->', type(ex).__name__ + ':', ex)


# An overflow, a non-number, and the wrong argument count each raise the CPython
# error.
show('exp2(1024)', lambda: math.exp2(1024.0))
show('cbrt(x)', lambda: math.cbrt('x'))
show('exp2()', lambda: math.exp2())
show('cbrt(1,2)', lambda: math.cbrt(1.0, 2.0))
