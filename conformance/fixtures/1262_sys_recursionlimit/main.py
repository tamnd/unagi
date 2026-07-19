import sys

# the default recursion limit CPython ships on 3.14
print(sys.getrecursionlimit())

# a new limit round-trips through the getter
sys.setrecursionlimit(1500)
print(sys.getrecursionlimit())
sys.setrecursionlimit(5000)
print(sys.getrecursionlimit())

# a limit below one is rejected, the stored value is left alone
for bad in (0, -5):
    try:
        sys.setrecursionlimit(bad)
    except ValueError as e:
        print("ve", e)
print(sys.getrecursionlimit())

# a non-integer is a TypeError
for bad in ("x", 1.5, None):
    try:
        sys.setrecursionlimit(bad)
    except TypeError as e:
        print("te", e)
