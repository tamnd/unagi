print(1 + 2j, 3 - 4j, 2j, complex(0, 0), complex(1, 0), complex(-1, 0))
print(repr(complex(0, -0.0)), repr(complex(-0.0, -0.0)), repr(complex(0, 1)), repr(complex(0, -1)))
print(repr(complex(1.5, 2.5)), repr(complex(2, -0.0)), repr(complex(-0.0, 2)))
print(0.5j, 1e300j, complex(float("inf"), float("nan")))

a = 1 + 2j
b = 3 + 4j
print(a + b, a - b, a * b, a / b)
print(a + 1, 1 + a, a + 1.5, a * 2, 2 * a, a - 1)
print(True + 1j, 1j + True, -a, +a)

print((2 + 3j) ** 2, (1 + 1j) ** 0, (1 + 2j) ** -1, (2j) ** 2, (0j) ** 2, (0j) ** 0)
print(abs(3 + 4j), abs(complex(0, 0)), abs(1j))

print(a.real, a.imag, a.conjugate(), (1j).conjugate())
print(hash(1 + 2j), hash(complex(1, 0)), hash(1 + 0j) == hash(1), hash(complex(0, 0)))

print(1 + 2j == complex(1, 2), complex(3, 0) == 3, complex(3, 0) == 3.0)
print((2 + 0j) == 2j, complex(1, 2) == complex(1, 2), 3 + 4j == 5)
print(bool(0j), bool(1j), bool(complex(0, 0)))

print(complex(1, 2), complex("1+2j"), complex("  3.5  "), complex("1"))
print(complex("j"), complex("1_000j"), complex("inf"), complex("nan+infj"))
print(complex("(1+2j)"), complex("+1+2j"), complex("-1-2j"), complex("1+j"), complex("1-j"))
print(complex(".5j"), complex("5.j"), complex("1.5e-3j"), complex("1e5"))
print(complex(real=1, imag=2), complex(imag=3), complex(1, imag=2), complex(True))
print(complex(1 + 2j))

print(f"{1 + 2j}", f"{3 - 4j!r}", str(2j), complex)

try:
    (1 + 2j) / 0
except ZeroDivisionError as e:
    print("zdiv:", e)
try:
    (0j) ** -1
except ZeroDivisionError as e:
    print("zpow:", e)
try:
    complex("1 + 2j")
except ValueError as e:
    print("bad:", e)
try:
    complex("abc")
except ValueError as e:
    print("bad2:", e)
try:
    (1 + 2j) < (3 + 4j)
except TypeError as e:
    print("ord:", e)
try:
    complex(None)
except TypeError as e:
    print("arg:", e)
try:
    complex("1", 2)
except TypeError as e:
    print("real:", e)
try:
    complex(1, "2")
except TypeError as e:
    print("imag:", e)
try:
    complex(1, 2, 3)
except TypeError as e:
    print("arity:", e)
