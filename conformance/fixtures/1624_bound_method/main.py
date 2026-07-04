# Bound-method and function introspection: __func__, __self__, the proxied
# __name__/__qualname__, and value equality/hash across repeated reads.

class C:
    def m(self):
        return 1

    def n(self):
        return 2


# A function read off the class carries its name and qualname.
print(C.m.__name__, C.m.__qualname__)
print(C.n.__name__, C.n.__qualname__)

c = C()
b1 = c.m
b2 = c.m

# __func__ is the underlying function, __self__ is the bound instance.
print(b1.__func__ is C.m, b1.__self__ is c)
print(b1.__name__, b1.__qualname__)

# Two reads are distinct objects but compare equal; a different method or a
# different instance does not.
print(b1 == b2, b1 is b2)
print(b1 == c.n, b1 == C().m)

# Equal bound methods hash alike and key the same dict slot.
print(hash(b1) == hash(b2))
d = {b1: "one", c.n: "two"}
print(d[b2], d[c.n], len(d))

# The unbound function is callable through __func__.
print(b1.__func__(c), b1())

# repr shape without the address.
print(repr(b1).startswith("<bound method C.m of "))

# A missing attribute reports the method type.
try:
    b1.nope
except AttributeError as e:
    print(str(e))
try:
    C.m.nope
except AttributeError as e:
    print(str(e))
