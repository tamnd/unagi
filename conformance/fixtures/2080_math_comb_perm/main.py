# math gains comb and perm, the two integer combinatorial counts. Both return
# exact big integers computed through the multiplicative recurrence, so their
# results are identical everywhere, unlike the transcendental routines. comb is
# the number of unordered selections and perm the number of ordered ones.

import math

print(math.comb(10, 3), math.comb(52, 5), math.comb(5, 0), math.comb(5, 5), math.comb(3, 5), math.comb(0, 0), math.comb(1000, 2))
print(math.perm(10, 3), math.perm(5), math.perm(5, 0), math.perm(3, 5), math.perm(0, 0), math.perm(1000, 3), math.perm(5, None))

# bool is an int subclass, so it counts as 0 or 1.
print(math.comb(True, True), math.perm(True))


def show(label, fn):
    try:
        print(label, '->', fn())
    except (TypeError, ValueError) as ex:
        print(label, '->', type(ex).__name__ + ':', ex)


# A negative argument, a non-integer, and a wrong argument count each raise the
# error CPython gives. perm with k omitted reports the factorial domain error
# for a negative n, while perm with k given reports the argument error.
show('comb(-1,2)', lambda: math.comb(-1, 2))
show('comb(5,-1)', lambda: math.comb(5, -1))
show('perm(-1)', lambda: math.perm(-1))
show('perm(5,-2)', lambda: math.perm(5, -2))
show('perm(-1,2)', lambda: math.perm(-1, 2))
show('comb(5,1.0)', lambda: math.comb(5, 1.0))
show('comb(1.0,1)', lambda: math.comb(1.0, 1))
show('perm(5,1.0)', lambda: math.perm(5, 1.0))
show('perm(1.0)', lambda: math.perm(1.0))
show('comb(5)', lambda: math.comb(5))
show('perm()', lambda: math.perm())
show('comb(1,2,3)', lambda: math.comb(1, 2, 3))
show('perm(1,2,3)', lambda: math.perm(1, 2, 3))
