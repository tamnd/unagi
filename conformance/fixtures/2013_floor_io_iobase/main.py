# _io._IOBase is the abstract base every stream derives from. Vendored io.py
# builds `class IOBase(_io._IOBase, metaclass=abc.ABCMeta)` on it, so the base
# has to carry the full default-method surface and inherit cleanly through an
# ABCMeta subclass. This is sub-slice 5b of the _io arc (Spec 2076 S0_io_arc.md);
# the concrete streams come later.
import _io
import abc
from _io import UnsupportedOperation

# The bare base: predicates are all false, the closed flag starts clear.
b = _io._IOBase()
print(b.closed, b.readable(), b.writable(), b.seekable(), b.isatty())
print(b.flush(), b._checkClosed())

# Operations a stream cannot perform raise UnsupportedOperation, tell is seek.
for m, arg in [("seek", (0,)), ("tell", ()), ("truncate", ()), ("fileno", ())]:
    try:
        getattr(b, m)(*arg)
    except UnsupportedOperation as e:
        print(m, "->", e)

# The _check* helpers raise UnsupportedOperation when the predicate is false.
for m in ["_checkReadable", "_checkWritable", "_checkSeekable"]:
    try:
        getattr(b, m)()
    except UnsupportedOperation as e:
        print(m, "->", e)

# close marks the stream closed, is idempotent, and shows the private flag.
print(b.close(), b.closed)
print(sorted(b.__dict__))
print(b.close(), b.closed)
for label, fn in [("flush", lambda: b.flush()),
                  ("_checkClosed", lambda: b._checkClosed())]:
    try:
        fn()
    except ValueError as e:
        print(label, "closed ->", e)

# io.py's own base: an ABCMeta subclass inherits the whole surface.
class IOBase(_io._IOBase, metaclass=abc.ABCMeta):
    __slots__ = ()

print([c.__name__ for c in IOBase.__mro__])

# A concrete subclass supplies read/readable and drives the mixin methods.
class Stream(IOBase):
    def __init__(self, data):
        self._d = data
        self._i = 0
    def readable(self):
        return True
    def writable(self):
        return True
    def read(self, n=-1):
        if n is None or n < 0:
            r = self._d[self._i:]
            self._i = len(self._d)
            return r
        r = self._d[self._i:self._i + n]
        self._i += len(r)
        return r

s = Stream(b"ab\ncd\n\nef")
print(s.readline(), s.readline(), s.readline(), s.readline(), s.readline())

s2 = Stream(b"x\ny\nz")
print(s2.readlines())

s3 = Stream(b"p\nq\n")
print(list(iter(s3)))

s4 = Stream(b"hello\nworld")
print(s4.readline(3), s4.readline(3))

# The context manager returns the stream and closes it, without suppressing.
s5 = Stream(b"")
with s5 as ctx:
    print("enter is self:", ctx is s5)
print("closed after with:", s5.closed)

s6 = Stream(b"")
try:
    with s6:
        raise KeyError("boom")
except KeyError:
    print("propagated, closed:", s6.closed)

# writelines drives write; a fresh stream records each piece.
class Sink(IOBase):
    def __init__(self):
        self.out = []
    def writable(self):
        return True
    def write(self, b):
        self.out.append(b)
        return len(b)

w = Sink()
w.writelines([b"a", b"bc", b"d"])
print(w.out)

# isinstance and register both work through the ABCMeta subclass.
print(isinstance(s, IOBase), isinstance(s, _io._IOBase))

@IOBase.register
class Faux:
    pass

print(issubclass(Faux, IOBase))
