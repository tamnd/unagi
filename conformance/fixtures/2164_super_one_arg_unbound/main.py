# super(T) with one argument builds an unbound super: a descriptor that carries
# the class but no instance until __get__ binds it. It resolves no cooperative
# names while unbound, exposes __thisclass__/__self__/__self_class__, and reprs
# with NULL for the missing instance.


class A:
    def m(self):
        return "A.m"


class B(A):
    def m(self):
        return "B.m"


u = super(B)
print(type(u).__name__)
print(repr(u))
print(u.__thisclass__.__name__, u.__self__, u.__self_class__)

# An unbound super has no cooperative attributes yet.
try:
    u.m
except AttributeError as e:
    print("unbound.m:", e)

# __get__ binds it to an instance, yielding the ordinary two-argument super.
b = B()
bound = u.__get__(b, B)
print(bound.m())
print(repr(bound))
print(bound.__self_class__.__name__)

# Binding to None returns the unbound super unchanged.
print(u.__get__(None, B) is u)

# The single argument must be a type.
try:
    super(5)
except TypeError as e:
    print("super(5):", e)


# An unbound super stored as a class attribute binds on instance access.
class C(B):
    def m(self):
        return "C.m"


C.sup = super(C)
c = C()
g = c.sup
print(type(g).__name__, g.__self_class__.__name__, g.m())
print(repr(C.__dict__['sup']))
