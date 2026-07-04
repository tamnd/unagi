# Three-argument type(name, bases, namespace): dynamic class creation.
C = type("C", (), {"x": 1, "greet": lambda self: "hi"})
print(C.__name__, C.__bases__, C.x)
c = C()
print(c.x, c.greet(), type(c).__name__, isinstance(c, C))

# a dynamic class inherits from a static base and keeps the MRO
class Base:
    def m(self):
        return "base"
D = type("D", (Base,), {"y": 2})
d = D()
print(d.m(), d.y, isinstance(d, Base))
print(D.__mro__)
print(issubclass(D, Base), issubclass(D, object))

# empty bases default to object, repr carries the module
E = type("E", (), {})
print(E.__bases__, repr(E))

# a namespace __module__ overrides the qualified name in repr
F = type("F", (), {"__module__": "mymod", "v": 9})
print(repr(F), F.v)

# methods defined in the namespace bind self and see each other
G = type("G", (), {
    "start": 10,
    "value": lambda self: self.start * 2,
    "label": lambda self: "G=" + str(self.value()),
})
g = G()
print(g.value(), g.label())

# dynamic and static classes compose either direction
H = type("H", (D,), {"z": 3})
h = H()
print(h.m(), h.y, h.z, [b.__name__ for b in H.__mro__])

# argument-type errors match type.__new__
for bad in (lambda: type("X", (), []),
            lambda: type("X", [], {}),
            lambda: type(5, (), {})):
    try:
        bad()
    except TypeError as e:
        print(e)

# an inconsistent base order raises the MRO conflict
class A:
    pass
class B(A):
    pass
try:
    type("Z", (A, B), {})
except TypeError as e:
    print(e)
