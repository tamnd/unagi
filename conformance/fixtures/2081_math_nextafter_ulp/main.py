# math gains nextafter and ulp, two exact IEEE operations. nextafter(x, y) is
# the next representable float after x in the direction of y, and ulp(x) is the
# gap between x and the next float up. Both are bit-exact against CPython, so the
# fixture asserts the values directly.

import math

# nextafter steps one representable value at a time.
print(math.nextafter(1.0, 2.0))
print(math.nextafter(1.0, 0.0))
print(math.nextafter(1.0, 1.0))
print(math.nextafter(0.0, 1.0))
print(math.nextafter(0.0, -1.0))
print(math.nextafter(-0.0, 1.0))

# Infinities and the edges of the range.
print(math.nextafter(math.inf, 0.0))
print(math.nextafter(-math.inf, 0.0))
print(math.nextafter(1e308, math.inf))
print(math.isnan(math.nextafter(math.nan, 1.0)))
print(math.nextafter(5.0, 5.0))

# ulp reports the least significant bit.
print(math.ulp(1.0))
print(math.ulp(2.0))
print(math.ulp(0.0))
print(math.ulp(-1.0))
print(math.ulp(1e308))
print(math.ulp(math.inf))
print(math.ulp(-math.inf))
print(math.isnan(math.ulp(math.nan)))

# bool and int arguments convert to float.
print(math.nextafter(True, 2))
print(math.ulp(1))


def show(label, fn):
    try:
        print(label, '->', fn())
    except (TypeError, ValueError) as ex:
        print(label, '->', type(ex).__name__ + ':', ex)


# A non-number and the wrong argument count each raise the CPython error.
show('nextafter(1)', lambda: math.nextafter(1.0))
show('ulp()', lambda: math.ulp())
show('nextafter(x,y)', lambda: math.nextafter('x', 1.0))
show('ulp(s)', lambda: math.ulp('s'))
