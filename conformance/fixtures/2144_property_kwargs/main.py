# property() accepts its arguments by keyword, not only by position. The stdlib's
# xml.dom.minidom builds every DOM attribute through a defproperty helper that
# calls property(fget, doc=...), so property() had to take fget, fset, fdel and
# doc as keywords for import xml.dom.minidom to work. __doc__ reads the explicit
# doc back, and falls back to the getter's own docstring when doc was not given,
# the way CPython copies fget.__doc__ into a property.


# doc= passed by keyword alongside a positional getter, the defproperty idiom.
class C:
    def _get_x(self):
        return 42

    x = property(_get_x, doc="the x value")


c = C()
print(c.x)
print(C.x.__doc__)


# With no doc, __doc__ falls back to the getter's own docstring.
class D:
    @property
    def y(self):
        "y docstring"
        return 1


print(D.y.__doc__)


# Every slot named by keyword.
class E:
    def g(self):
        return 7

    def s(self, v):
        pass

    z = property(fget=g, fset=s, doc="z")


e = E()
print(e.z)
print(E.z.__doc__)
e.z = 9


# No doc and no getter docstring reads back as None.
class F:
    @property
    def w(self):
        return 2


print(F.w.__doc__)


# The setter chain keeps the doc the getter property carried.
class G:
    def __init__(self):
        self._v = 0

    @property
    def v(self):
        "v doc"
        return self._v

    @v.setter
    def v(self, value):
        self._v = value


g = G()
g.v = 5
print(g.v)
print(G.v.__doc__)


# An unknown keyword is rejected.
try:
    property(bogus=1)
except TypeError:
    print("TypeError")
