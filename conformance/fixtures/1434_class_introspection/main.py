# A class exposes its identity through __name__ and __qualname__ and its
# ancestry through __bases__, __mro__, and __base__. The implicit object root
# shows up in the tuples even when no base was written, and a root class reports
# object as its single base.
class Base:
    pass


class Mid(Base):
    pass


class Leaf(Mid):
    pass


print(Leaf.__name__)
print(Leaf.__qualname__)
print(Base.__name__)

print(Base.__bases__)
print(Leaf.__bases__)
print(Leaf.__mro__)

print(Leaf.__base__.__name__)
print(Base.__base__.__name__)


class A:
    pass


class B:
    pass


class C(A, B):
    pass


print(C.__bases__)
print(C.__mro__)
print(C.__base__.__name__)
