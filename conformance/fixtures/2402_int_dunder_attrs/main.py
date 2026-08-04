# int and bool expose their operator and special dunders as readable attributes,
# the same additive way complex and float do. The operators still evaluate
# through the normal arithmetic; reading a slot only hands back a callable.

names = ["__abs__", "__bool__", "__divmod__", "__rdivmod__", "__round__",
         "__getnewargs__", "__hash__", "__floor__", "__ceil__",
         "__neg__", "__pos__", "__invert__", "__add__", "__index__"]
print("int  ", [n for n in names if hasattr(5, n)] == names)
print("bool ", [n for n in names if hasattr(True, n)] == names)

# The sign-free magnitude and the truth value.
print((-7).__abs__(), (0).__abs__(), (10 ** 30).__abs__() == 10 ** 30)
print((0).__bool__(), (5).__bool__(), (True).__bool__())

# divmod carries Python's floor/mod sign, not a truncated pair.
print((7).__divmod__(3), (7).__divmod__(-3))
print((-7).__divmod__(3), (-7).__divmod__(-3))
print((7).__rdivmod__(20))
print((7).__divmod__("x"))

# floor and ceil round an int to itself, getnewargs rebuilds it.
print((5).__floor__(), (5).__ceil__(), (-3).__floor__())
print((5).__getnewargs__(), (True).__getnewargs__())

# round leaves a non-negative or absent digit count alone and rounds a negative
# one to a power of ten with ties going to the even multiple.
print((12345).__round__(), (12345).__round__(None), (12345).__round__(2))
print((12345).__round__(-2), (12345).__round__(-3), (12345).__round__(-4))
print((15).__round__(-1), (25).__round__(-1), (35).__round__(-1))
print((-15).__round__(-1), (-25).__round__(-1))
print((5).__round__(-100), (0).__round__(-3))

# The value hash matches the builtin, and a big int spills without loss.
print((5).__hash__(), (True).__hash__(), (-7).__hash__() == hash(-7))

# The slots read back off the type as unbound methods too.
print(int.__abs__(-8), int.__bool__(0), int.__divmod__(7, 3))
print(int.__round__(15, -1), int.__floor__(9), int.__getnewargs__(4))

# The error paths match CPython's wording.
def show(f):
    try:
        print("ok", f())
    except Exception as e:
        print(type(e).__name__, e)

show(lambda: (5).__abs__(1))
show(lambda: (5).__bool__(2))
show(lambda: (5).__round__(1.5))
show(lambda: (5).__round__(1, 2))
show(lambda: (5).__divmod__())
show(lambda: (5).__divmod__(0))
show(lambda: int.__abs__("x"))
