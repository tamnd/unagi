# T.__new__(T, value) read off a builtin type object is the constructor, and
# bool.__new__ returns the True/False singleton. CPython's tp_new safety rules
# reject borrowing a base's __new__ for a different type.
print(int.__new__(int, 5), int.__new__(int))
print(str.__new__(str, "x"), repr(str.__new__(str)))
print(bytes.__new__(bytes, b"ab"))
print(tuple.__new__(tuple, [1, 2, 3]))

# bool.__new__(bool, x) is the singleton, identical to the literal.
print(bool.__new__(bool) is False)
print(bool.__new__(bool, 1) is True)
print(bool.__new__(bool, 0) is False)
print(bool.__new__(bool, False) is False)
print(bool.__new__(bool, True) is True)
print(type(bool.__new__(bool, 1)).__name__)

# The safety matrix: same type builds, a subtype with its own __new__ is not
# safe, a non-subtype is rejected.
cases = [
    ("int.__new__(bool, 0)", lambda: int.__new__(bool, 0)),
    ("bool.__new__(int, 0)", lambda: bool.__new__(int, 0)),
    ("int.__new__(str, 0)", lambda: int.__new__(str, 0)),
]
for name, fn in cases:
    try:
        print(name, "->", repr(fn()))
    except TypeError as e:
        print(name, "-> TypeError:", e)

# A user subclass still allocates the subclass instance through __new__.
class MyInt(int):
    pass
m = int.__new__(MyInt, 7)
print(type(m).__name__, int(m))
