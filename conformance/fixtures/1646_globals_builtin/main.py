# globals() returns the module namespace as an ordinary dict. Reading a name,
# testing membership, and iterating the keys all work, and type(globals()) is
# dict. The dict is built from the names bound when the call runs, so each call
# reflects the module state at that point.
x = 5
y = "hi"


def foo():
    return 1


class C:
    pass


import math

z = foo() + int(math.pi)

print(type(globals()) is dict)
print("x" in globals(), "foo" in globals(), "missing" in globals())
print(globals()["x"], globals()["y"], globals()["z"])
print(globals()["foo"] is foo)
print(globals()["C"] is C)
print(sorted(n for n in globals() if not n.startswith("_")))


# globals() reaches the module namespace from inside a function too.
def inside():
    return sorted(n for n in globals() if not n.startswith("_"))


print(inside())

# The result is a real dict, so the dict methods work on it.
print(sorted(k for k in globals().keys() if not k.startswith("_")))
print(globals().get("x"), globals().get("nope", "default"))

# A missing key raises KeyError like any dict.
try:
    globals()["nope"]
except KeyError as e:
    print("KeyError:", e)

# A name assigned after a call is absent from that call's dict but present in a
# later one.
print("later" in globals())
later = 1
print("later" in globals())
