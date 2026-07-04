# type() over every builtin value kind: repr and self-identity.
values = [
    5, True, 1.0, "s", b"b", bytearray(b"x"),
    [1], (1,), {1}, frozenset({1}), {1: 2}, range(3), 1j, None, ...,
]
for v in values:
    t = type(v)
    print(repr(t), type(v) is t)

# Constructor identity: type(x) is the same object as the builtin name.
print(type(5) is int, type(True) is bool, type(1.0) is float)
print(type("") is str, type([]) is list, type(()) is tuple)
print(type({}) is dict, type(set()) is set, type(b"") is bytes)
print(type(range(0)) is range, type(1j) is complex)

# The metatype: a type's type is type, and type is its own type.
print(type(int) is type, type(str) is type, type(type) is type)
print(repr(type), type.__name__)


class Animal:
    pass


class Dog(Animal):
    pass


d = Dog()
print(type(d) is Dog, type(d) is Animal)
print(type(Dog) is type, type(d).__name__)
print(repr(type(d)))

# __name__ and __qualname__ on type objects and builtin functions.
print(int.__name__, int.__qualname__)
print(type(None).__name__, type(...).__name__)
print(len.__name__)

# A returned type is callable as its constructor.
print(type("x")("42"))
print(type([])((1, 2, 3)))
print(type(5)("100"))

# Functions and builtins report their own type.
def f():
    pass


print(repr(type(f)), repr(type(len)))

# type() arity: 1 or 3 only.
for n in (0, 2, 4):
    try:
        type(*([1] * n))
    except TypeError as e:
        print(n, e)
