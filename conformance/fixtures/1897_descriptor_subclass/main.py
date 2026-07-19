# Subclassing the classmethod, staticmethod and property builtins, the shape
# abc's abstractclassmethod, abstractstaticmethod and abstractproperty take: the
# subclass instance wraps the descriptor and delegates the descriptor protocol
# to it.


class my_classmethod(classmethod):
    pass


class my_staticmethod(staticmethod):
    pass


class my_property(property):
    pass


class C:
    tag = "base"

    @my_classmethod
    def whoami(cls):
        return cls.tag

    @my_staticmethod
    def add(a, b):
        return a + b

    @my_property
    def doubled(self):
        return self._n * 2

    @doubled.setter
    def doubled(self, value):
        self._n = value // 2

    @doubled.deleter
    def doubled(self):
        self._n = 0

    def __init__(self):
        self._n = 5


# A classmethod subclass binds the class it is reached through, on the class and
# through an instance, and a subclass sees its own type.
class D(C):
    tag = "derived"


print(C.whoami(), D.whoami(), C().whoami(), D().whoami())

# A staticmethod subclass hands back the bare function, no self or cls bound.
print(C.add(2, 3), C().add(4, 5))

# The wrappers really are subclass instances, not plain builtins.
print(type(C.__dict__["whoami"]).__name__, isinstance(C.__dict__["whoami"], classmethod))
print(type(C.__dict__["add"]).__name__, isinstance(C.__dict__["add"], staticmethod))
print(type(C.__dict__["doubled"]).__name__, isinstance(C.__dict__["doubled"], property))

# A property subclass runs getter, setter and deleter through the instance.
c = C()
print(c.doubled)
c.doubled = 30
print(c.doubled, c._n)
del c.doubled
print(c.doubled, c._n)


# A read-only property subclass raises on write, matching property.
class RO:
    @my_property
    def x(self):
        return 42


ro = RO()
print(ro.x)
try:
    ro.x = 1
except AttributeError as e:
    print("no setter:", type(e).__name__)
