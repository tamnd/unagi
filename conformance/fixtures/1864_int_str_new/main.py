# int and str answer __new__ off the type object as their own allocator, the
# one an enum member creation calls as str.__new__(cls, value).


class MyStr(str):
    pass


class MyInt(int):
    pass


# The allocator is the type's own, distinct from object's.
print(int.__new__ is object.__new__)
print(str.__new__ is object.__new__)
print(int.__new__ is int.__new__)
print(str.__new__ is str.__new__)

# Called on a subclass with a value it builds an instance of that subclass
# carrying the builtin payload.
s = str.__new__(MyStr, "hi")
print(type(s).__name__, repr(s), isinstance(s, MyStr))

n = int.__new__(MyInt, 42)
print(type(n).__name__, n, isinstance(n, MyInt))

# The payload behaves as the underlying builtin value.
print(s + " there")
print(n + 1)
