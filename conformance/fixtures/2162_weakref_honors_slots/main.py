# A class supports weak references only when its layout carries __weakref__:
# a plain (__dict__-bearing) class does, a __slots__ class only when it declares
# __weakref__ or inherits it. weakref.ref and weakref.proxy enforce the same rule.
# Strong references are held so the deref tests weak support, not GC timing.
import weakref


class Plain:
    pass


class WithWeak:
    __slots__ = ('a', '__weakref__')


class NoWeak:
    __slots__ = ('a',)


class SubNoWeak(NoWeak):
    __slots__ = ('b',)


class SubGetsWeak(NoWeak):
    pass


p = Plain()
print(weakref.ref(p)() is p)

w = WithWeak()
print(weakref.ref(w)() is w)

for cls in (NoWeak, SubNoWeak):
    obj = cls()
    try:
        weakref.ref(obj)
        print(cls.__name__, "NO ERROR")
    except TypeError as e:
        print(cls.__name__, "TypeError:", e)

s = SubGetsWeak()
print(weakref.ref(s)() is s)

n = NoWeak()
try:
    weakref.proxy(n)
    print("proxy NO ERROR")
except TypeError as e:
    print("proxy TypeError:", e)
