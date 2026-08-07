import math

# Zero-dimensional and single-axis points.
print(math.dist((), ()))
print(math.dist((3,), (0,)))
print(math.dist((2,), (5,)))

# Pythagorean distances across dimensions.
print(math.dist((0, 0), (3, 4)))
print(math.dist((0, 0), (5, 12)))
print(math.dist((1, 2, 3), (4, 6, 3)))
print(math.dist((0, 0, 0, 0), (3, 4, 12, 84)))

# Correctly rounded fractional coordinates.
print(math.dist((0.1, 0.2), (0.4, 0.6)))
print(math.dist((1.5, 2.5, 3.5), (0.0, 0.0, 0.0)))
print(math.dist((3.141592653589793,), (2.718281828459045,)))

# Wide magnitudes: a naive sum of squared differences overflows or underflows,
# scaling keeps these finite and correctly rounded.
print(math.dist((0, 0), (1e308, 1e308)))
print(math.dist((0, 0, 0), (1e200, 1e200, 1e200)))
print(math.dist((0, 0), (1e-200, 1e-200)))
print(math.dist((0, 0), (5e-324, 5e-324)))

# Overflow with no infinite coordinate still returns inf, no exception.
print(math.dist((0, 0), (-1.5e308, 1.5e308)))

# C99 special values screen on the coordinate differences: an infinite
# difference wins even against a nan difference.
print(math.dist((float('inf'), 0.0), (0.0, float('nan'))))
print(math.dist((float('nan'), 0.0), (0.0, 0.0)))
print(math.dist((float('inf'), 0.0), (float('inf'), 0.0)))

# Mismatched dimensions raise ValueError.
try:
    math.dist((1, 2), (1,))
except ValueError as e:
    print("ValueError:", e)
