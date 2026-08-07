import math
from fractions import Fraction
from decimal import Decimal

# Empty and exact-integer dot products stay exact.
print(math.sumprod([], []))
print(math.sumprod([1, 2, 3], [4, 5, 6]))
print(math.sumprod([10**30, 2], [10**30, 3]))

# Compensated float summation: a naive sum of products rounds these one place
# off, the triple-length accumulator lands on the correctly rounded double.
print(math.sumprod([0.3, 100.0, 100.0], [0.0, -0.7, 1.5]))
print(math.sumprod([1.0, 0.5, 0.000123, -0.7], [7.0, -0.7, 2.5, 7.0]))
print(math.sumprod([0.1, 0.2, 0.3], [0.1, 0.2, 0.3]))

# The natural mixed cases the fast path targets: price times quantity and a
# boolean mask, integers folded into the float run as doubles.
print(math.sumprod([2.5, 3.0, 4.25], [3, 2, 4]))
print(math.sumprod([True, False, True], [2.5, 3.5, 4.5]))

# A big integer ahead of the floats keeps every term on exact object arithmetic.
print(math.sumprod([10**30, 0.5, 3], [2, 4.0, 3]))

# Overflow returns inf with no infinite input, mirroring the float sum.
print(math.sumprod([1e308, 1e308], [10.0, 10.0]))

# The object fallback carries the general numeric types through their own
# arithmetic, so the result type and value follow the operands.
print(repr(math.sumprod([1 + 2j, 3 - 1j], [3 + 4j, 2 + 0j])))
print(repr(math.sumprod([Fraction(1, 3), 2], [3, Fraction(5, 2)])))
print(repr(math.sumprod([Decimal('1.5'), Decimal('2.25')], [Decimal('2'), Decimal('4')])))

# Unequal lengths raise ValueError.
try:
    math.sumprod([1, 2], [1])
except ValueError as e:
    print("ValueError:", e)
