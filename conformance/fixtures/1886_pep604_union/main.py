# PEP 604 type unions, the X | Y form. copyreg builds one at import time with
# pickle(type(int | str), pickle_union), so the operator has to produce a union,
# type() of it has to be typing.Union, and the union has to work as a dict key.
# The rest is the language feature: repr, __args__, set-based equality, isinstance
# membership, flattening, deduplication, and the collapse of a single member back
# to the type itself.

print(int | str)
print(str | int)
print(int | str == str | int)
print(int | str == int | bytes)

# a single distinct member is just that type, not a union.
print(int | int)
print(int | str | int)

# nested unions flatten, None joins as NoneType but prints as None.
print((int | str) | bytes)
print(int | None)
print((int | str) | (bytes | None))

print(type(int | str))
print((int | str).__args__)
print((int | None).__args__)

print(isinstance(3, int | str), isinstance(b"x", int | str))
print(isinstance(None, int | None), isinstance(1.0, int | None))


class C:
    pass


print(C | int)
print(isinstance(C(), C | int), isinstance("s", C | int))

# set-based hashing: equal unions share a dict slot regardless of order.
table = {int | str: "either", bytes: "raw"}
print(table[str | int])
print((int | str) in table)

# a union member that is not a type is the type.__or__ TypeError.
try:
    int | 5
except TypeError as e:
    print("type error:", e)
