# complex exposes its operator and special dunders as readable callables, the
# same additive attribute surface int carries, so a bound read and a direct call
# agree and hasattr matches CPython.
c = 1 + 2j

# Forward and reflected arithmetic dunders.
print(c.__add__(3), c.__add__(1.5), c.__add__(1j), c.__add__("x"))
print(c.__radd__(3), c.__rsub__(3), c.__rtruediv__(2))
print(c.__sub__(1j), c.__mul__(2), c.__truediv__(1j))
print(c.__pow__(2), (2 + 0j).__rpow__(3), c.__pow__(2, None))

# Unary slots and magnitude.
print(c.__neg__(), c.__pos__(), c.__abs__())

# Rich comparison declines a non-numeric operand with NotImplemented.
print(c.__eq__(1 + 2j), c.__eq__(3), c.__eq__("x"))
print(c.__ne__(1 + 2j), c.__ne__(3), c.__ne__("x"))

# Truth, hash, repr, str and format.
print(c.__bool__(), (0j).__bool__())
print(c.__hash__(), (3 + 0j).__hash__())
print(c.__repr__(), c.__str__(), c.__format__(""))
print(repr(c.__format__(">10")), repr((1.5 + 2.5j).__format__(".2f")))

# Pickling and coercion helpers.
print(c.__getnewargs__())
print(c.__complex__(), c.__complex__() is c)
print(c.conjugate(), complex.conjugate(c))

# The attribute surface: the operator dunders resolve, the ones complex lacks
# stay absent.
print(hasattr(c, "__add__"), hasattr(c, "__abs__"), hasattr(complex, "conjugate"))
print(hasattr(c, "__floordiv__"), hasattr(c, "__float__"), hasattr(c, "__index__"))

# abs of a finite pair that overflows raises, an infinite part does not.
print(abs(complex(1e308, 1e308)))
try:
    abs(complex(1.7e308, 1.7e308))
except OverflowError as e:
    print("OverflowError", e)
print(abs(complex(float("inf"), 1)))


def show(f):
    try:
        print(f())
    except Exception as e:
        print(type(e).__name__, e)


# Argument-count and modulo errors match the C slot wrappers.
show(lambda: c.__pow__(2, 3))
show(lambda: c.__abs__(1))
show(lambda: c.__add__())
show(lambda: c.__format__(5))
show(lambda: c.__getnewargs__(1))
show(lambda: c.__complex__(1))
show(lambda: c.nope)
