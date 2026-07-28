# int and bool expose their arithmetic and bitwise operator dunders, both bound
# on an instance and unbound off the type, the way CPython does. This is what
# lets multiprocessing.reduction read type(int.__add__) at import. The `+`
# operator itself still evaluates through the fast path; this is the readable
# attribute surface beside it.

# Forward binary slots compute the operator.
print((5).__add__(3))
print((5).__sub__(2))
print((6).__mul__(7))
print((10).__floordiv__(3))
print((10).__mod__(3))
print((2).__pow__(8))
print((1).__lshift__(4))
print((32).__rshift__(2))
print((6).__and__(3))
print((6).__or__(1))
print((6).__xor__(3))

# __truediv__ produces a float the way / does.
print((5).__truediv__(2))

# Reflected slots compute other <op> self.
print((5).__radd__(3))
print((5).__rsub__(2))

# A forward slot declines a non-int operand with NotImplemented, so the protocol
# hands the pair to the other operand rather than int claiming it. A float is
# declined too: int does not add a float, float.__radd__ does the work.
print((5).__add__("x"))
print((5).__mul__(2.0))

# The unary slots negate, keep, and invert.
print((5).__neg__())
print((5).__pos__())
print((5).__invert__())

# bool inherits int's slots, being a subtype.
print(True.__add__(1))
print(True.__mul__(0))

# The slot reads back off the type as the unbound method: int.__add__(5, 3)
# matches (5).__add__(3), and its __name__ is the slot name.
print(int.__add__(5, 3))
print(bool.__add__(True, 2))
print(int.__add__.__name__)

# hasattr sees the slot on both an instance and the type.
print(hasattr(5, "__add__"))
print(hasattr(int, "__mul__"))
