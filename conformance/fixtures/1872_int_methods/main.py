# Every int carries its integer methods and number attributes, shared with
# bool because a bool is an int, each returning a plain int over the value's
# rational view n/1.

for n in (0, 5, 255, -255, 10 ** 40):
    print(n, n.bit_length(), n.bit_count(), n.conjugate(), n.is_integer())
    print(n.as_integer_ratio(), n.numerator, n.denominator, n.real, n.imag)

print(True.bit_length(), True.bit_count(), True.numerator, True.real, True.imag)
print(type(True.real).__name__, type(True.numerator).__name__)

k = 6
print(k.bit_length())

try:
    (5).bit_length(1)
except TypeError as e:
    print(e)
