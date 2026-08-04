# A float literal that overflows the IEEE-754 double range folds to an infinity
# at parse time (1e400 is inf, -1e400 is -inf), and an imaginary literal that
# overflows folds the same way (1e400j is infj). These have no Go literal
# spelling, so the compiler lowers them through a runtime helper rather than an
# invalid +Inf token. The folded value carries the sign, compares and arithmetics
# like any infinity, and round-trips through math.isinf and repr.
import math

print(1e400)
print(-1e400)
print(1E400)
print(1e400 == 2e400)
print(1e400 > 1e308)
print(math.isinf(1e400))
print(math.isinf(-1e400))
print(1e400 - 1e400)  # inf - inf is nan
print(1e400 * 0)      # inf * 0 is nan
print(1e-400)         # underflows to 0.0, stays finite
print(repr(1e400))

# An overflowing imaginary literal folds to an infinite imaginary part.
c = 1e400j
print(c)
print(c.imag)
print(complex(0, -1e400))

# The folded infinity works as a constant inside an expression and a container.
values = [1e400, -1e400, 1e400 - 1e400]
print(values)
print(max(1.0, 1e400))
print(min(-1e400, -1.0))

# A hugely overflowing exponent still folds cleanly.
print(1e999999)
print(-1e999999)
