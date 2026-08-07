import math

# Zero and single-argument forms.
print(math.hypot())
print(math.hypot(0.0, 0.0, 0.0))
print(math.hypot(3))
print(math.hypot(-3))

# Pythagorean triples: exact integer results across arities.
print(math.hypot(3, 4))
print(math.hypot(5, 12))
print(math.hypot(8, 15))
print(math.hypot(3, 4, 12))
print(math.hypot(3, 4, 12, 84))

# Small integer pairs where a naive libm hypot loses the last place.
print(math.hypot(2, 3))
print(math.hypot(1, 4))
print(math.hypot(7, 11))

# Correctly rounded fractional coordinates.
print(math.hypot(0.1, 0.1))
print(math.hypot(1.5, 2.5, 3.5))
print(math.hypot(3.141592653589793, 2.718281828459045))

# Wide magnitudes: a naive sum of squares overflows or underflows to inf/0,
# scaling keeps these finite and correctly rounded.
print(math.hypot(1e308, 1e308))
print(math.hypot(1e200, 1e200, 1e200))
print(math.hypot(1e-200, 1e-200))
print(math.hypot(5e-324, 5e-324))

# C99 special values: an infinity wins even against a nan.
print(math.hypot(float('inf'), float('nan')))
print(math.hypot(float('nan'), float('inf')))
print(math.hypot(-float('inf'), 2.0))
print(math.hypot(float('nan'), 1.0))
print(math.hypot(float('nan'), float('nan')))

# Overflow with no infinite input still returns inf, no exception.
print(math.hypot(1.5e308, 1.5e308))
