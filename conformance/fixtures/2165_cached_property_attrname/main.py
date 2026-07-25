import functools


class C:
    def __init__(self, r):
        self.r = r

    @functools.cached_property
    def area(self):
        return self.r * self.r


print(C.area.attrname)
print(C.__dict__["area"].attrname)
c = C(3)
print(c.area)
print(C.area.attrname)
print(C.area.func.__name__)

unassigned = functools.cached_property(lambda s: 1)
print(unassigned.attrname)

try:

    class D:
        cp = functools.cached_property(lambda s: 1)
        x = cp
        y = cp

except TypeError as e:
    print(type(e).__name__ + ": " + str(e))
