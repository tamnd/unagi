import math


def show(label, fn):
    try:
        print(label, "->", repr(fn()))
    except Exception as e:
        print(label, "|", type(e).__name__, "|", e)


# The default is a single step, and an explicit None matches it.
show("default", lambda: math.nextafter(1.0, 2.0))
show("none", lambda: math.nextafter(1.0, 2.0, steps=None))

# A steps count walks that many representable floats towards y.
show("steps3_up", lambda: math.nextafter(1.0, 2.0, steps=3))
show("steps3_down", lambda: math.nextafter(1.0, 0.0, steps=3))
show("steps0", lambda: math.nextafter(1.0, 2.0, steps=0))
show("steps_bool", lambda: math.nextafter(1.0, 2.0, steps=True))
show("steps_equal", lambda: math.nextafter(1.0, 1.0, steps=5))
show("steps_overshoot", lambda: math.nextafter(1.0, math.inf, steps=2))

# A walk that crosses zero flips the sign at the smallest subnormal.
show("cross_up", lambda: math.nextafter(0.0, math.inf, steps=1))
show("cross_from_neg", lambda: math.nextafter(-0.0, math.inf, steps=1))
show("cross_subnormal", lambda: math.nextafter(-1e-323, 1.0, steps=5))
show("cross_down", lambda: math.nextafter(1e-323, -1.0, steps=10))

# A count past the uint64 ceiling saturates and lands on y.
show("saturate", lambda: math.nextafter(1.0, 2.0, steps=10 ** 30))
show("saturate_huge", lambda: math.nextafter(1.0, 2.0, steps=10 ** 400))

# A nan on either side passes through.
show("nan_x", lambda: math.nextafter(math.nan, 1.0, steps=3))
show("nan_y", lambda: math.nextafter(1.0, math.nan, steps=3))

# Error and signature paths.
show("neg_steps", lambda: math.nextafter(1.0, 2.0, steps=-1))
show("float_steps", lambda: math.nextafter(1.0, 2.0, steps=2.0))
show("str_steps", lambda: math.nextafter(1.0, 2.0, steps="a"))
show("positional_steps", lambda: math.nextafter(1.0, 2.0, 3))
show("one_arg", lambda: math.nextafter(1.0))
show("four_args", lambda: math.nextafter(1.0, 2.0, 3, 4))
show("bad_kw", lambda: math.nextafter(1.0, 2.0, foo=3))
show("keyword_xy", lambda: math.nextafter(x=1.0, y=2.0))
show("x_not_real", lambda: math.nextafter("a", 2.0))
