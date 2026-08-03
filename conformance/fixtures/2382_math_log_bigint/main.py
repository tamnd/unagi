import math


def show(label, fn):
    try:
        print(label, "->", repr(fn()))
    except Exception as e:
        print(label, "|", type(e).__name__, "|", e)


# The value cases are rounded, because an irrational logarithm lands on the
# last-ULP difference between Go's math and the system libm CPython links, and
# that difference varies by platform. Rounding keeps the fixture stable while
# still proving the log came back finite and correct rather than infinite.
def approx(fn):
    return round(fn(), 6)


# Logs of ints too large to convert to a double must stay finite. CPython works
# the exact int through a mantissa/exponent split rather than letting the float
# conversion overflow to infinity.
big = 10 ** 1000
show("log_big", lambda: approx(lambda: math.log(big)))
show("log2_big", lambda: approx(lambda: math.log2(big)))
show("log10_big", lambda: approx(lambda: math.log10(big)))
show("log_big_base10", lambda: approx(lambda: math.log(big, 10)))
show("log_big_base2", lambda: approx(lambda: math.log(big, 2)))
show("log10_10pow", lambda: approx(lambda: math.log10(10 ** 500)))
show("log_hugebase", lambda: approx(lambda: math.log(10 ** 600, 10 ** 300)))
show("log_biggest", lambda: approx(lambda: math.log(1 << 100000)))
show("log_big_finite", lambda: math.isfinite(math.log(big)))

# log2 of an exact power of two is exact on any platform, since log2(0.5) is
# -1 and log2(2.0) is 1 with no rounding.
show("log2_2pow", lambda: math.log2(1 << 2000))
show("log2_exact", lambda: math.log2(2 ** 60))

# Ints that still fit a double take the ordinary path.
show("log_int", lambda: approx(lambda: math.log(123456789)))
show("log10_1000", lambda: approx(lambda: math.log10(1000)))
show("log_1", lambda: math.log(1))
show("log2_1", lambda: math.log2(1))

# A non-positive int is a domain error that does not quote the (possibly huge)
# value, while a non-positive float quotes its repr.
show("log_negint", lambda: math.log(-5))
show("log_zeroint", lambda: math.log(0))
show("log_negbig", lambda: math.log(-(10 ** 1000)))
show("log2_negint", lambda: math.log2(-5))
show("log10_zeroint", lambda: math.log10(0))
show("log_negfloat", lambda: math.log(-5.0))
show("log_zerofloat", lambda: math.log(0.0))
show("log2_negfloat", lambda: math.log2(-2.5))
show("log10_negfloat", lambda: math.log10(-1.0))

# Argument-count errors keep CPython's wording.
show("log_noargs", lambda: math.log())
show("log_3args", lambda: math.log(1, 2, 3))

# Base combinations on ordinary values.
show("log_base", lambda: approx(lambda: math.log(8, 2)))
show("log_base_e", lambda: approx(lambda: math.log(math.e)))
