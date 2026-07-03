a = 12
b = 10
print(a & b, a | b, a ^ b)
print(a << 2, a >> 2)
print(~a, ~-1, -~5)
print(1 | 2 ^ 3 & 4)
print((1 << 8) - 1)
print(3 & 6 == 2, (3 & 6) == 2, 3 & (6 == 2))
print(True | False, True & True, True ^ True)
print(True << 3)
x = 5
x |= 2
x &= 6
x ^= 3
x <<= 2
x >>= 1
print(x)
try:
    print(1 << -1)
except ValueError as e:
    print("caught", e)
try:
    print(1 | "a")
except TypeError as e:
    print("caught", e)
try:
    print(~1.5)
except TypeError as e:
    print("caught", e)
