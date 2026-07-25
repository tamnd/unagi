# __new__ is an implicit staticmethod, so it appears in a class __dict__ wrapped,
# and cls.__dict__['__new__'].__func__ recovers the underlying function. The
# enum machinery relies on this to build members, so http and pstats import.


class C:
    def __new__(cls, *args):
        return super().__new__(cls)

    def __init__(self, x=0):
        self.x = x


# Instantiation and __init__ run as before: the internal slot is the plain
# function, so wrapping the reflective view changes nothing here.
c = C(5)
print(c.x)

# The class dict presents __new__ as a staticmethod carrying the function.
nm = C.__dict__["__new__"]
print(type(nm).__name__)
print(nm.__func__ is C.__dict__["__new__"].__func__)
print(callable(nm.__func__))

# Reading the attribute off the class still yields the plain function, the way
# staticmethod.__get__ hands it back.
print(type(C.__new__).__name__)

# A class that defines no __new__ has none in its own dict; it is inherited.
class D:
    pass


print("__new__" in D.__dict__)
print("__new__" in C.__dict__)

# super().__new__ still resolves through a subclass that forwards arguments.
class E(C):
    def __new__(cls, *args):
        return super().__new__(cls, *args)

    def __init__(self, x=0):
        self.x = x * 2


e = E(9)
print(e.x)
