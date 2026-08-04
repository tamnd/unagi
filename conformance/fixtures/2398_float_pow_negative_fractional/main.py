# A negative base raised to a non-integral power is complex, matching CPython's
# float.__pow__ which falls through to complex power instead of returning nan.
# The irrational parts ride the Go-vs-libm last-ULP ceiling the cmath and math
# ports already document, so the complex results are rounded before printing.
from fractions import Fraction as F


def rc(z, n=9):
    if isinstance(z, complex):
        return complex(round(z.real, n), round(z.imag, n))
    return round(z, n)


def show(label, fn):
    try:
        print(label, repr(rc(fn())))
    except Exception as e:
        print(label, type(e).__name__, str(e))


# Negative base, non-integral exponent: complex.
show("cbrt", lambda: (-8.0) ** (1 / 3))
show("cbrt27", lambda: (-27.0) ** (1 / 3))
show("sqrt-1", lambda: (-1.0) ** 0.5)
show("sqrt-2", lambda: (-2.0) ** 0.5)
show("pow1.5", lambda: (-4.0) ** 1.5)
show("negexp", lambda: (-8.0) ** -0.5)
show("intcbrt", lambda: (-8) ** (1 / 3))
show("builtincbrt", lambda: pow(-8.0, 1 / 3))

# Fraction delegates to float power, so a negative Fraction base is complex too.
show("fraction", lambda: F(-8) ** F(1, 3))

# Integral exponents on a negative base stay real.
show("square", lambda: (-8.0) ** 2.0)
show("cube", lambda: (-8.0) ** 3.0)
show("zeroexp", lambda: (-8.0) ** 0.0)
show("intneg", lambda: (-8.0) ** -1.0)

# Non-negative base stays real.
show("pos-sqrt", lambda: (2.0) ** 0.5)
show("pos-cbrt", lambda: (8.0) ** (1 / 3))
show("zerobase", lambda: (0.0) ** 0.5)

# Infinite base and infinite or nan exponent keep CPython's real handling.
show("infexp+", lambda: (-2.0) ** float("inf"))
show("infexp-", lambda: (-2.0) ** float("-inf"))
show("infbase", lambda: float("-inf") ** 0.5)
show("nanexp", lambda: (-2.0) ** float("nan"))

# Error path: zero to a negative power is unchanged.
show("zeroneg", lambda: (0.0) ** -1.0)
