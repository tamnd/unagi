# __slots__ replaces the instance dict with fixed member descriptors: unlisted
# attributes have nowhere to land, unset slots raise, and the layout composes
# under inheritance with CPython's conflict rules.

class C:
    __slots__ = ('x', 'y')

c = C()
try:
    c.z = 1
except AttributeError as e:
    print("new attr:", e)
try:
    print(c.x)
except AttributeError as e:
    print("unset read:", e)
c.x = 5
print("x:", c.x)
del c.x
try:
    print(c.x)
except AttributeError as e:
    print("read after del:", e)
try:
    del c.y
except AttributeError as e:
    print("del unset:", e)
try:
    del c.nope
except AttributeError as e:
    print("del missing:", e)
try:
    print(c.__dict__)
except AttributeError as e:
    print("no dict:", e)
print("descr:", C.x)
print("slots kept:", C.__slots__)


class One:
    __slots__ = 'only'

o = One()
o.only = 1
print("string slots:", o.only)
try:
    o.other = 2
except AttributeError as e:
    print("string slots err:", e)


class D(C):
    pass

d = D()
d.anything = 9
print("plain subclass gets dict:", d.anything, d.__dict__)


class E(C):
    __slots__ = ('z',)

e = E()
e.x = 1
e.z = 2
print("slots stack:", e.x, e.z)
try:
    e.w = 3
except AttributeError as e2:
    print("slots subclass:", e2)


try:
    class Bad:
        __slots__ = ('v',)
        v = 1
except ValueError as ex:
    print("class var conflict:", ex)

try:
    class BadMethod:
        __slots__ = ('m',)
        def m(self):
            pass
except ValueError as ex:
    print("method conflict:", ex)

try:
    class BadName:
        __slots__ = (1,)
except TypeError as ex:
    print("non-str:", ex)


class WithDict:
    __slots__ = ('a', '__dict__')

w = WithDict()
w.a = 1
w.b = 2
print("dict in slots:", w.a, w.b, w.__dict__)


class Priv:
    __slots__ = ('__secret',)

    def put(self):
        self.__secret = 7
        return self.__secret

p = Priv()
print("mangled:", p.put())
print("has mangled descr:", hasattr(Priv, '_Priv__secret'))


class Base:
    __slots__ = ('v',)

class Sub(Base):
    __slots__ = ()

s = Sub()
s.v = 4
print("inherited slot:", s.v)
try:
    s.other = 1
except AttributeError as ex:
    print("empty slots sub:", ex)


class DictBase:
    pass

class SlotsSub(DictBase):
    __slots__ = ('k',)

x = SlotsSub()
x.free = 1
x.k = 2
print("dict base keeps dict:", x.free, x.k, x.__dict__)


class A:
    __slots__ = ('a',)

class B:
    __slots__ = ('b',)

try:
    class AB(A, B):
        pass
except TypeError as ex:
    print("layout conflict:", ex)


class D1:
    __slots__ = ('__dict__',)

try:
    class D2(D1):
        __slots__ = ('__dict__',)
except TypeError as ex:
    print("dup dict slot:", ex)

try:
    class D3(DictBase):
        __slots__ = ('__dict__',)
except TypeError as ex:
    print("dict slot over dict base:", ex)


class RO:
    __slots__ = ('r',)

RO.r = 10
ro = RO()
print("class attr after overwrite:", ro.r)
try:
    ro.r = 1
except AttributeError as ex:
    print("read only:", ex)


class Dup:
    __slots__ = ('q', 'q')

dq = Dup()
dq.q = 3
print("dup slot ok:", dq.q)


class Mixed(DictBase):
    __slots__ = ('mv',)

mx = Mixed()
mx.__dict__['mv'] = 'dictval'
mx.mv = 'slotval'
print("descr wins over dict:", mx.mv, mx.__dict__)


class InInit:
    __slots__ = ('name', 'size')

    def __init__(self, name, size):
        self.name = name
        self.size = size

    def grow(self):
        self.size += 1
        return self.size

ii = InInit("box", 2)
print("init through slots:", ii.name, ii.grow(), ii.size)
