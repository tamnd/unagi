# math gains prod, the product counterpart of the builtin sum. prod(iterable)
# multiplies every element together, and the keyword-only start seeds the
# accumulator, so an empty iterable returns start unchanged.

import math

# Ordinary products over a few sequence types.
print(math.prod([1, 2, 3, 4]))
print(math.prod((2, 3, 5)))
print(math.prod(range(1, 6)))
print(math.prod([]))
print(math.prod([7]))

# start seeds the accumulator and defaults to 1.
print(math.prod([1, 2, 3], start=10))
print(math.prod([], start=5))
print(math.prod([2, 3], start=2.0))

# The elements can be floats, and the result stays a float.
print(math.prod([1.5, 2.0, 4.0]))

# A generator is consumed like any iterable.
print(math.prod(x * x for x in range(1, 5)))

# bool is an int subclass, so it multiplies as 0 or 1.
print(math.prod([True, True, True]))
print(math.prod([True, False, True]))


def show(label, fn):
    try:
        print(label, '->', fn())
    except TypeError as ex:
        print(label, '->', type(ex).__name__ + ':', ex)


# A non-iterable, an unexpected keyword, a positional start, and too many
# positionals each raise the CPython error.
show('prod(5)', lambda: math.prod(5))
show('prod()', lambda: math.prod())
show('prod(x,bad=1)', lambda: math.prod([1, 2], bad=1))
show('prod(x,2)', lambda: math.prod([1, 2], 2))
