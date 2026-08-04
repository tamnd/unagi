import math


class MyInt(int):
    pass


class Big(int):
    pass


class Idx:
    def __index__(self):
        return 7


class BadIdx:
    def __index__(self):
        return "nope"


# The integer math routines coerce through __index__ the way CPython does, so an
# int subclass reads its stored value and any object spelling __index__ converts.
a = MyInt(12)
print(math.gcd(a, 8), math.gcd(18, a), math.gcd(a, a), math.gcd(a))
print(math.lcm(a, 8), math.lcm(a, MyInt(9)), math.lcm())
print(math.comb(a, 3), math.comb(a, MyInt(3)), math.perm(a, 2), math.perm(a, MyInt(0)))
print(math.factorial(MyInt(5)), math.factorial(MyInt(0)))
print(math.isqrt(MyInt(16)), math.isqrt(MyInt(0)))

# A user object with __index__ works everywhere an integer is required.
print(math.gcd(Idx(), 21), math.lcm(Idx(), 3))
print(math.comb(Idx(), 2), math.perm(Idx(), Idx()))
print(math.factorial(Idx()), math.isqrt(Idx()))

# A big int subclass keeps full precision through the coercion.
print(math.gcd(Big(10 ** 30), Big(10 ** 20)))
print(math.factorial(MyInt(21)))

# The same __index__ coercion reaches range, chr, the base reprs, subscription
# and slicing through the shared index path.
print(list(range(a - 9)), list(range(MyInt(2), MyInt(6))), list(range(0, 10, MyInt(3))))
print(chr(a + 53), hex(a), bin(a), oct(a))
print([10, 20, 30, 40][MyInt(2)], (1, 2, 3, 4)[MyInt(-1)], "abcd"[MyInt(1)])
print([0, 1, 2, 3, 4, 5][MyInt(1):MyInt(4)], "abcdef"[MyInt(0):MyInt(6):MyInt(2)])
print(list(range(Idx())), hex(Idx()), "abcdefgh"[Idx()])

# A non-integer without __index__ still raises the same TypeError, and a bad
# __index__ return type is rejected with CPython's wording.
for bad in [1.5, "3", None, 2j]:
    try:
        math.gcd(bad, 2)
    except TypeError as e:
        print("gcd", type(bad).__name__, e)
try:
    math.factorial(1.5)
except TypeError as e:
    print("factorial", e)
try:
    math.isqrt(2.0)
except TypeError as e:
    print("isqrt", e)
try:
    math.gcd(BadIdx(), 2)
except TypeError as e:
    print("badidx", e)
