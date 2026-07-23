# The math module gains the four C99 special functions gamma, lgamma, erf and
# erfc. Their transcendental values track the platform libm to the last bit, so
# this fixture asserts only the facts every libm agrees on: the exact
# integer-point values, the infinities, the domain errors, and the argument
# checks. random and statistics reach lgamma and erfc through these names.

import math

# gamma at integer points is exactly the factorial (n-1)!, identical everywhere.
print([math.gamma(n) for n in (1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 10.0)])

# lgamma at 1 and 2 is exactly 0.0.
print(math.lgamma(1.0), math.lgamma(2.0))

# erf and erfc at 0 and at the infinities are exact.
print(math.erf(0.0), math.erf(float('inf')), math.erf(float('-inf')))
print(math.erfc(0.0), math.erfc(float('inf')), math.erfc(float('-inf')))
print(math.gamma(float('inf')))
print(math.lgamma(float('inf')), math.lgamma(float('-inf')))

# gamma and lgamma are undefined at the non-positive integers and at negative
# infinity, where both raise the same domain error.
for f in ('gamma', 'lgamma'):
    fn = getattr(math, f)
    for bad in (0.0, -1.0, -5.0, float('-inf')):
        try:
            fn(bad)
        except ValueError as e:
            print(f, repr(bad), 'VE:', e)

# A finite argument whose gamma overflows is a range error.
try:
    math.gamma(200.0)
except OverflowError as e:
    print('gamma OE:', e)

# The argument checks match CPython, which qualifies the name with the module.
try:
    math.gamma()
except TypeError as e:
    print('TE:', e)
try:
    math.erf('x')
except TypeError as e:
    print('TE:', e)
