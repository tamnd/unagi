# math gains fma, the fused multiply-add x*y + z rounded once, and isclose, the
# tolerance comparison. fma is an exact IEEE operation so it matches CPython
# bit for bit, and isclose returns a plain bool from a deterministic test, so the
# fixture asserts the outcomes on values that sit clearly inside or outside the
# tolerance, never on a boundary.

import math

# fma rounds x*y + z once. On these small exact values that equals the ordinary
# expression.
print(math.fma(2.0, 3.0, 4.0))
print(math.fma(0.5, 0.5, 0.0))
print(math.fma(-2.0, 3.0, 1.0))
print(math.fma(1e16, 1.0, 1.0))

# int and bool arguments convert to float.
print(math.fma(2, 3, 4))
print(math.fma(True, 3, 4))

# a nan from a nan input passes straight through.
print(math.isnan(math.fma(math.nan, 1.0, 1.0)))
print(math.isnan(math.fma(1.0, 1.0, math.nan)))

# an infinite factor with a finite result stays infinite.
print(math.fma(math.inf, 2.0, 1.0))

# isclose is exactly equal values, then values inside and outside the default
# relative tolerance of 1e-09.
print(math.isclose(1.0, 1.0))
print(math.isclose(1.0, 1.0 + 1e-12))
print(math.isclose(1.0, 1.1))

# the keyword-only tolerances widen the test.
print(math.isclose(0.0, 1e-12, abs_tol=1e-9))
print(math.isclose(100.0, 101.0, rel_tol=0.02))
print(math.isclose(100.0, 101.0, rel_tol=0.005))

# matching infinities are close, a lone infinity and any nan are not.
print(math.isclose(math.inf, math.inf))
print(math.isclose(math.inf, -math.inf))
print(math.isclose(math.inf, 1.0))
print(math.isclose(math.nan, math.nan))


def show(label, fn):
    try:
        print(label, '->', fn())
    except (TypeError, ValueError, OverflowError) as ex:
        print(label, '->', type(ex).__name__ + ':', ex)


# fma raises on an invalid 0 times infinity, on an overflow from finite inputs,
# on a non-number, and on the wrong argument count.
show('fma(0,inf,1)', lambda: math.fma(0.0, math.inf, 1.0))
show('fma(inf,0,1)', lambda: math.fma(math.inf, 0.0, 1.0))
show('fma overflow', lambda: math.fma(1e308, 10.0, 0.0))
show('fma(x)', lambda: math.fma('x', 1.0, 1.0))
show('fma(2,3)', lambda: math.fma(2.0, 3.0))

# isclose rejects a negative tolerance, a missing argument, and an unknown
# keyword.
show('isclose negtol', lambda: math.isclose(1.0, 1.0, rel_tol=-1e-9))
show('isclose negabs', lambda: math.isclose(1.0, 1.0, abs_tol=-1.0))
show('isclose 1 arg', lambda: math.isclose(1.0))
show('isclose 3 pos', lambda: math.isclose(1.0, 2.0, 3.0))
show('isclose badkw', lambda: math.isclose(1.0, 1.0, foo=1))
