# math gains fsum, an accurate floating-point sum. It keeps a list of
# nonoverlapping partial sums, so a run of values that a naive left-to-right sum
# would round badly comes back correctly rounded. The results are exact IEEE
# doubles, so the fixture asserts them directly.

import math

# The classic case: a naive sum of these loses the small terms, fsum keeps them.
print(math.fsum([1e16, 1.0, -1e16]))

# Ten tenths sum to exactly 1.0 rather than 0.9999999999999999.
print(math.fsum([0.1] * 10))

# Cancelling large magnitudes and the half-even case across partials.
print(math.fsum([1e-16, 1.0, 1e16]))
print(math.fsum([1.0, 1e100, 1.0, -1e100]))

# Empty and single-element sums, and a plain integer run.
print(math.fsum([]))
print(math.fsum([42.0]))
print(math.fsum(range(1, 11)))

# A generator is consumed like any iterable.
print(math.fsum(1.0 / n for n in range(1, 6)))

# Infinities and nan flow through the special-value handling.
print(math.fsum([1.0, math.inf, 2.0]))
print(math.fsum([1.0, -math.inf]))
print(math.isnan(math.fsum([1.0, math.nan])))


def show(label, fn):
    try:
        print(label, '->', fn())
    except (TypeError, ValueError, OverflowError) as ex:
        print(label, '->', type(ex).__name__ + ':', ex)


# Mixed infinities, an intermediate overflow from finite terms, a non-number
# element, and the wrong argument count each raise the CPython error.
show('inf-inf', lambda: math.fsum([math.inf, -math.inf]))
show('overflow', lambda: math.fsum([1e308, 1e308]))
show('bad elem', lambda: math.fsum([1.0, 'x']))
show('no args', lambda: math.fsum())
