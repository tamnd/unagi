# math.ldexp takes any integer exponent, an arbitrary-precision int or a bool, not
# just one that fits a machine word. A huge exponent is not a parse error: it
# clamps so the value overflows to a math range error or underflows to a signed
# zero, while a zero, infinity or NaN base is returned unchanged whatever the
# exponent. A non-integer exponent stays the ldexp TypeError.
import math


def show(label, fn):
    try:
        print(label, "=>", repr(fn()))
    except Exception as e:
        print(label, "=>", type(e).__name__, ":", e)


big = 10 ** 19

show("small", lambda: math.ldexp(1.0, 3))
show("bool_exp", lambda: math.ldexp(1.0, True))
show("bigint_base", lambda: math.ldexp(10 ** 30, 3))

# Huge positive exponent overflows, huge negative underflows to a signed zero.
show("overflow", lambda: math.ldexp(1.0, big))
show("overflow_neg", lambda: math.ldexp(-1.0, big))
show("underflow", lambda: math.ldexp(1.0, -big))
show("underflow_neg", lambda: math.ldexp(-1.0, -big))

# A special base is returned unchanged for any exponent.
show("inf", lambda: math.ldexp(math.inf, -big))
show("ninf", lambda: math.ldexp(-math.inf, big))
show("zero", lambda: math.ldexp(0.0, big))
show("nzero", lambda: math.ldexp(-0.0, big))
show("nan", lambda: math.ldexp(math.nan, big))

# A normal-range exponent that still overflows the double is a range error.
show("double_overflow", lambda: math.ldexp(1e308, 5))

# A non-integer exponent is the ldexp TypeError.
show("float_exp", lambda: math.ldexp(1.0, 2.0))
show("str_exp", lambda: math.ldexp(1.0, "3"))
