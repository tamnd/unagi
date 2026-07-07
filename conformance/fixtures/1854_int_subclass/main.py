class MyInt(int):
    pass


a = MyInt(5)
b = MyInt(3)

# Arithmetic returns a plain int; the subclass does not propagate.
print(a + b, a - b, a * b, a // b, a % b, a**b)
print(type(a + b).__name__)

# Bit operations.
print(a & b, a | b, a ^ b, ~a, a << 1, a >> 1)

# Unary.
print(-a, +a, abs(MyInt(-7)))

# Comparisons against plain ints and other instances.
print(a == 5, a < 10, a > b, a >= 5, a != 5)

# isinstance and identity of the class.
print(isinstance(a, int), isinstance(a, MyInt), type(a).__name__)

# Conversions.
print(int(a), float(a), bool(MyInt(0)), bool(a))

# Hashing matches the underlying int, so instances key like their value.
print(hash(a) == hash(5))
d = {MyInt(1): "one"}
print(d[1])

# String forms and formatting specs read through to the int.
print(str(a), repr(a), f"{a}", format(a, "04d"))
print(bin(a), hex(MyInt(255)), format(a, "b"))

# Index contexts: subscript and range.
print([0, 1, 2, 3, 4][MyInt(2)])
print(list(range(MyInt(3))))


class Scaled(int):
    def __new__(cls, value):
        return super().__new__(cls, value * 10)


s = Scaled(2)
print(s, s + 5, type(s).__name__)


# A subclass that overrides an operator dunder keeps its override, the way an
# IntFlag keeps __or__ over the inherited int arithmetic.
class Flag(int):
    def __or__(self, other):
        return Flag(int(self) | int(other))


f = Flag(1) | Flag(4)
print(int(f), type(f).__name__)

