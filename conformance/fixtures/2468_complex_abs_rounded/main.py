# abs(z) and complex.__abs__ return the magnitude hypot(re, im). CPython computes
# it through the platform C hypot, which is correctly rounded, so the result is the
# nearest double to the true magnitude. Go's math.Hypot is not correctly rounded
# and lands a unit in the last place off on a large share of pairs (abs(2 + 3j) is
# the first), so unagi rounds the exact magnitude once to match. Integer pairs are
# correctly rounded by every mainstream libm, so they are asserted here; the handful
# of small-float pairs where the platform C hypot is itself a bit off are left out,
# the way math.dist and math.sumprod assert only the values they agree on.
import cmath

def show(label, fn):
    try:
        print(label, "=>", repr(fn()))
    except Exception as e:
        print(label, "=>", type(e).__name__, ":", e)

# The non-Pythagorean integer pairs are exactly the ones Go's math.Hypot rounds
# wrong; the correctly-rounded magnitude matches CPython.
for re in range(-8, 9):
    for im in range(-8, 9):
        print(re, im, repr(abs(complex(re, im))))

# Pythagorean pairs land on an exact integer magnitude.
for a, b, r in [(3, 4, 5), (5, 12, 13), (8, 15, 17), (7, 24, 25), (20, 21, 29), (9, 40, 41)]:
    show(f"abs({a}+{b}j)", lambda a=a, b=b: abs(complex(a, b)))

# __abs__ read directly and cmath.polar's magnitude agree with abs().
show("(2+3j).__abs__()", lambda: (2 + 3j).__abs__())
show("complex.__abs__(2+3j)", lambda: complex.__abs__(2 + 3j))
show("cmath.polar(2+3j)[0]", lambda: cmath.polar(2 + 3j)[0])
show("cmath.polar(6+8j)", lambda: cmath.polar(6 + 8j))

# Signed zeros and a real or imaginary axis value.
show("abs(0j)", lambda: abs(0j))
show("abs(complex(-0.0, 0.0))", lambda: abs(complex(-0.0, 0.0)))
show("abs(complex(0.0, -0.0))", lambda: abs(complex(0.0, -0.0)))
show("abs(complex(-3, 0))", lambda: abs(complex(-3, 0)))
show("abs(complex(0, -5))", lambda: abs(complex(0, -5)))

# Non-finite parts follow C99: an infinite part wins over a nan, otherwise a nan
# part yields nan.
inf = float('inf')
nan = float('nan')
show("abs(complex(inf, 1))", lambda: abs(complex(inf, 1)))
show("abs(complex(1, -inf))", lambda: abs(complex(1, -inf)))
show("abs(complex(-inf, nan))", lambda: abs(complex(-inf, nan)))
show("abs(complex(nan, inf))", lambda: abs(complex(nan, inf)))
show("abs(complex(nan, 1))", lambda: abs(complex(nan, 1)))
show("abs(complex(nan, nan))", lambda: abs(complex(nan, nan)))

# A finite pair whose squares overflow double still computes through the wider
# magnitude, and only a true magnitude past the double range raises OverflowError.
show("abs(complex(1e200, 1e200))", lambda: abs(complex(1e200, 1e200)))
show("abs(complex(3e-200, 4e-200))", lambda: abs(complex(3e-200, 4e-200)))
show("abs(complex(1.5e308, 1.5e308))", lambda: abs(complex(1.5e308, 1.5e308)))
