# An inherited builtin method read off a Python subclass instance carries the
# bound-method identity CPython gives it: __self__ is the instance, __qualname__
# joins the subclass qualname to the method name, and __name__ is bare.
def show(label, fn):
    try:
        print(label, repr(fn()))
    except Exception as e:
        print(label, type(e).__name__, ":", e)


class D(dict):
    pass


class L(list):
    pass


class S(set):
    pass


class T(str):
    pass


d = D(a=1, b=2)
show("D.get.__self__ is d", lambda: d.get.__self__ is d)
show("D.get.__qualname__", lambda: d.get.__qualname__)
show("D.get.__name__", lambda: d.get.__name__)
show("D.items.__qualname__", lambda: d.items.__qualname__)

l = L([1, 2, 3])
show("L.append.__self__ is l", lambda: l.append.__self__ is l)
show("L.append.__qualname__", lambda: l.append.__qualname__)
show("L.append.__name__", lambda: l.append.__name__)

s = S([1, 2])
show("S.add.__self__ is s", lambda: s.add.__self__ is s)
show("S.add.__qualname__", lambda: s.add.__qualname__)

t = T("hi")
show("T.upper.__self__ is t", lambda: t.upper.__self__ is t)
show("T.upper.__qualname__", lambda: t.upper.__qualname__)

import array


class A(array.array):
    pass


a = A("i", [1, 2])
show("A.append.__self__ is a", lambda: a.append.__self__ is a)
show("A.append.__qualname__", lambda: a.append.__qualname__)

from collections import Counter

c = Counter("aab")
show("Counter.get.__self__ is c", lambda: c.get.__self__ is c)
show("Counter.get.__qualname__", lambda: c.get.__qualname__)

# The bound method still calls through to the payload.
show("d.get('a')", lambda: d.get("a"))
l.append(4)
show("l after append", lambda: list(l))
