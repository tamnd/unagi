# The builtin sum switched to Neumaier compensated summation for floats in
# CPython 3.12, so a run of floats and ints that a naive left-to-right sum would
# round badly now comes back correctly rounded. The results are exact IEEE
# doubles, so the fixture asserts them directly.

# The classic cancellation: the small term survives.
print(sum([1e16, 1.0, -1e16]))

# Ten tenths sum to exactly 1.0.
print(sum([0.1] * 10))

# A mix of ints and floats folds through the same compensated path, and the
# int-typed start is carried until the first float appears.
print(sum([1, 2, 3, 0.5]))
print(sum([1e16, 1, 1, -1e16]))
print(sum([0.1] * 10, 0.0))

# bool counts as an int in the compensated path.
print(sum([1e16, True, True, -1e16]))

# A pure-int sum stays an exact int, never converted to float.
print(sum([10 ** 30, 1, -10 ** 30]))
print(sum(range(1, 101)))

# Infinities and nan behave like ordinary float addition.
print(sum([1e308, 1e308, -1e308]))
print(sum([1.0, float('inf'), 2.0]))
r = sum([1.0, float('nan')])
print(r != r)

# An empty sum returns the start unchanged.
print(sum([]))
print(sum([], 5.0))

# A non-number element after floats falls through to plain addition and raises
# the ordinary TypeError.
try:
    sum([1.0, 'x'])
except TypeError as ex:
    print('bad ->', type(ex).__name__)
