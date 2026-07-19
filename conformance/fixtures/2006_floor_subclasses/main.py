# type.__subclasses__() reports a class's direct subclasses in creation order.
# _py_abc.__subclasscheck__ walks `for scls in cls.__subclasses__()` to find a
# registered virtual subclass, so an ABC cannot run its check without it. unagi
# tracks each child on its bases as the class is built and answers the bound
# zero-argument method from that list.
class A:
    pass


class B(A):
    pass


class C(A):
    pass


class D(B):
    pass


print("A:", [c.__name__ for c in A.__subclasses__()])
print("B:", [c.__name__ for c in B.__subclasses__()])
print("C:", [c.__name__ for c in C.__subclasses__()])
print("D:", [c.__name__ for c in D.__subclasses__()])

# The read returns a fresh list, and the elements are the class objects
# themselves, so identity holds.
subs = A.__subclasses__()
print("fresh list:", subs is A.__subclasses__())
print("identity:", subs[0] is B, subs[1] is C)

# Multiple inheritance registers the child on every base.
class E(B, C):
    pass


print("B after E:", [c.__name__ for c in B.__subclasses__()])
print("C after E:", [c.__name__ for c in C.__subclasses__()])

# A class created after an earlier read shows up on the next call, so the list
# is live rather than a snapshot.
before = len(A.__subclasses__())


class F(A):
    pass


print("A grew:", len(A.__subclasses__()) == before + 1)
