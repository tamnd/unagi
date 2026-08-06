from fractions import Fraction


class MyInt(int):
    pass


class Big(int):
    pass


# An int subclass inherits the named int methods off its payload, each reading
# the stored value and returning a plain int the way CPython's int methods do.
a = MyInt(12)
print(a.bit_length(), a.bit_count(), a.conjugate(), a.is_integer())
print(a.as_integer_ratio(), MyInt(-40).as_integer_ratio())
print(a.to_bytes(3, "big"), a.to_bytes(3, "little"), MyInt(255).to_bytes(2, "big"))
print(MyInt(0).bit_length(), MyInt(0).bit_count(), MyInt(-6).bit_count())

# The read-only rational view (numerator, denominator, real, imag) reads off the
# payload and comes back as a plain int, matching CPython's Integral registration.
print(a.numerator, a.denominator, a.real, a.imag)
print(type(a.numerator).__name__, type(a.conjugate()).__name__, type(a.real).__name__)
print(MyInt(-7).numerator, MyInt(-7).denominator, MyInt(-7).real, MyInt(-7).imag)

# The motivating case: Fraction over an int subclass reads the inherited
# numerator and denominator rather than raising on the missing attribute.
print(Fraction(MyInt(3), MyInt(4)))
print(Fraction(MyInt(6), MyInt(8)))
print(Fraction(a))
print(Fraction(MyInt(10), MyInt(4)) + Fraction(1, 2))

# A big int subclass keeps full precision through the payload read.
big = Big(10 ** 40)
print(big.bit_length(), big.numerator, big.denominator)
print(big.bit_count(), (big + 1).bit_length())

# A class-level override still shadows the inherited member, so the payload
# fallback never masks a user method or property.
class Over(int):
    def bit_length(self):
        return 999

    @property
    def numerator(self):
        return -1


o = Over(5)
print(o.bit_length(), o.numerator, o.denominator, o.bit_count())

# The reconstruction tuple int exposes for pickle resolves through the payload.
print(a.__getnewargs__(), MyInt(-3).__getnewargs__())

# Arithmetic on the subclass still returns a plain int, unchanged by the attrs.
print(a + 1, type(a + 1).__name__, a * MyInt(2), type(a * MyInt(2)).__name__)
