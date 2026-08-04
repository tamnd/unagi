# float exposes its arithmetic and unary operator dunders as readable callables,
# the same additive attribute surface int and bool carry. The operators still
# route through Add, Pow and friends, so this only makes the slots readable. The
# comparison dunders already resolved through the scalar-compare path; this closes
# the arithmetic, unary and number-protocol slots.
f = 3.5

# Forward and reflected arithmetic. A non-float operand declines NotImplemented.
print(f.__add__(2), f.__add__(2.0), f.__add__(True), f.__add__(1j), f.__add__("x"))
print(f.__radd__(2), f.__rsub__(2), f.__rtruediv__(7))
print(f.__sub__(1), f.__mul__(2), f.__truediv__(2))
print(f.__floordiv__(2), f.__rfloordiv__(10), f.__mod__(2), f.__rmod__(10))
print(f.__divmod__(2), f.__rdivmod__(10), (-3.5).__divmod__(2))

# Power stays real for an exact case and hands a negative base with a fractional
# exponent to complex, the value CPython's float_pow falls through to. The modulo
# argument is only allowed as None.
print(f.__pow__(2), (2.0).__rpow__(3), f.__pow__(2, None))
cube = (-8.0).__pow__(1 / 3)
print(type(cube).__name__, round(cube.real, 9), round(cube.imag, 9))

# Unary slots, truth, hash and pickling helper.
print((-3.5).__neg__(), f.__pos__(), (-3.5).__abs__())
print(f.__bool__(), (0.0).__bool__())
print(f.__hash__(), (2.0).__hash__())
print(f.__getnewargs__())

# The attribute surface: the arithmetic dunders resolve, the ones float lacks
# stay absent.
print(hasattr(f, "__add__"), hasattr(f, "__divmod__"), hasattr(f, "__abs__"))
print(hasattr(f, "__and__"), hasattr(f, "__index__"))


def show(g):
    try:
        print(g())
    except Exception as e:
        print(type(e).__name__, e)


# Argument-count, modulo and zero-division errors match the C slot wrappers.
show(lambda: f.__neg__(1))
show(lambda: f.__hash__(1))
show(lambda: f.__getnewargs__(1))
show(lambda: f.__add__())
show(lambda: f.__add__(1, 2))
show(lambda: f.__divmod__())
show(lambda: f.__pow__(2, 3))
show(lambda: f.__pow__(2, 3, 4))
show(lambda: f.__rpow__(2, 3))
show(lambda: f.__truediv__(0))
show(lambda: f.__mod__(0))
