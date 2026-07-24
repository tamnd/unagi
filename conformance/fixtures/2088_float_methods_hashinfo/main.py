import fractions, gettext, sys


def show(label, fn):
    try:
        print(label, '->', fn())
    except (TypeError, ValueError, OverflowError, AttributeError) as ex:
        print(label, '->', type(ex).__name__ + ':', ex)


# float attributes and the number-protocol methods, all read from IEEE bits so
# they hold identically on every host.
show("real", lambda: (3.5).real)
show("imag", lambda: (3.5).imag)
show("conjugate", lambda: (3.5).conjugate())
show("numerator", lambda: (3.5).numerator)

show("is_integer 3.0", lambda: (3.0).is_integer())
show("is_integer 3.5", lambda: (3.5).is_integer())
show("is_integer inf", lambda: float('inf').is_integer())
show("is_integer arg", lambda: (3.0).is_integer(1))

show("air 0.25", lambda: (0.25).as_integer_ratio())
show("air 3.0", lambda: (3.0).as_integer_ratio())
show("air -0.5", lambda: (-0.5).as_integer_ratio())
show("air 0.1", lambda: (0.1).as_integer_ratio())
show("air inf", lambda: float('inf').as_integer_ratio())
show("air nan", lambda: float('nan').as_integer_ratio())
show("air arg", lambda: (3.0).as_integer_ratio(1))

show("hex 3.5", lambda: (3.5).hex())
show("hex 1.0", lambda: (1.0).hex())
show("hex 0.0", lambda: (0.0).hex())
show("hex -0.0", lambda: (-0.0).hex())
show("hex 0.1", lambda: (0.1).hex())
show("hex inf", lambda: float('inf').hex())
show("hex nan", lambda: float('nan').hex())
show("hex arg", lambda: (3.5).hex(1))

show("trunc 3.7", lambda: (3.7).__trunc__())
show("trunc -3.7", lambda: (-3.7).__trunc__())
show("floor -3.2", lambda: (-3.2).__floor__())
show("ceil -3.2", lambda: (-3.2).__ceil__())
show("int 3.7", lambda: (3.7).__int__())
show("int 1e30", lambda: (1e30).__int__())
show("int inf", lambda: float('inf').__int__())
show("floor nan", lambda: float('nan').__floor__())
show("float 3.7", lambda: (3.7).__float__())

# fractions builds on float.as_integer_ratio and int arithmetic.
F = fractions.Fraction
print("frac int", F(1, 2) + F(1, 3))
print("frac 0.25", F(0.25))
print("frac 0.1", F(0.1))

# gettext needs sys.base_prefix; the null translation is an identity map.
print("gettext", gettext.gettext('hello world'))

# sys.hash_info carries the 64-bit host constants, identical on every host. The
# prefix paths themselves are host-specific so only the null gettext lookup,
# which needs sys.base_prefix to resolve, is asserted above.
hi = sys.hash_info
print("hash_info", hi.width, hi.modulus, hi.inf, hi.nan, hi.imag,
      hi.algorithm, hi.hash_bits, hi.seed_bits, hi.cutoff)
