class A: pass
class B: pass
class C(A): pass

# Classes as set elements.
s = {A, B}
print("A in s:", A in s)
print("C in s:", C in s)
print("len s:", len(s))

# hash is stable across reads.
print("hash stable:", hash(A) == hash(A))

# set algebra over classes.
print("union:", len({A, B} | {B, C}))
print("intersection is {B}:", ({A, B} & {B, C}) == {B})

# frozenset of classes.
fs = frozenset({A, B})
print("A in fs:", A in fs)
print("C in fs:", C in fs)

# Classes as dict keys, mixed with builtin types.
reg = {int: "i", str: "s", A: "a", B: "b"}
print("reg[int]:", reg[int])
print("reg[A]:", reg[A])
print("reg[B]:", reg[B])

# A class inside a tuple key.
d = {(A, 1): "a1", (B, 2): "b2"}
print("d[(A,1)]:", d[(A, 1)])

# type() of an instance keys back to the class.
td = {type(A()): "inst-of-A"}
print("td[A]:", td[A])

# Instances in a set stay identity-hashed and distinct.
a1, a2 = A(), A()
iset = {a1, a2, a1}
print("len iset:", len(iset))
print("a1 in iset:", a1 in iset)

# ABCMeta-style registry driven by a real set of classes.
class Meta(type):
    def __init__(cls, n, b, ns):
        super().__init__(n, b, ns)
        cls.members = set()
    def register(cls, x):
        cls.members.add(x)
        return x
    def __subclasscheck__(cls, sub):
        return sub in cls.members

class Iface(metaclass=Meta):
    pass

Iface.register(A)
print("issubclass(A, Iface):", issubclass(A, Iface))
print("issubclass(B, Iface):", issubclass(B, Iface))
