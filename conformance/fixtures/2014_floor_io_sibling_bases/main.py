# _io._RawIOBase, _io._BufferedIOBase and _io._TextIOBase are the three sibling
# abstract bases every concrete stream derives from. Vendored io.py builds
# RawIOBase/BufferedIOBase/TextIOBase on them with metaclass=abc.ABCMeta, so each
# has to inherit the _IOBase surface and carry its own default-raise methods.
# This is sub-slice 5c of the _io arc (Spec 2076 stdlib S0_io_arc.md); the
# concrete streams come later.
import _io
import abc
from _io import UnsupportedOperation

# Each sibling base derives from _IOBase and inherits its predicates.
for name in ["_RawIOBase", "_BufferedIOBase", "_TextIOBase"]:
    cls = getattr(_io, name)
    print(name, [b.__name__ for b in cls.__bases__])
    b = cls()
    print("  bare:", b.closed, b.readable(), b.writable(), b.seekable())

# _RawIOBase leaves readinto and write unimplemented as NotImplementedError, and
# read and readall funnel through readinto, so they raise it too.
r = _io._RawIOBase()
for label, fn in [("readinto", lambda: r.readinto(bytearray(4))),
                  ("write", lambda: r.write(b"x")),
                  ("read", lambda: r.read(5)),
                  ("readall", lambda: r.readall())]:
    try:
        fn()
    except NotImplementedError:
        print("raw", label, "-> NotImplementedError")

# _BufferedIOBase raises UnsupportedOperation; readinto surfaces read, readinto1
# surfaces read1.
bf = _io._BufferedIOBase()
for label, fn in [("read", lambda: bf.read()),
                  ("read1", lambda: bf.read1()),
                  ("write", lambda: bf.write(b"x")),
                  ("detach", lambda: bf.detach()),
                  ("readinto", lambda: bf.readinto(bytearray(4))),
                  ("readinto1", lambda: bf.readinto1(bytearray(4)))]:
    try:
        fn()
    except UnsupportedOperation as e:
        print("buf", label, "->", e)

# _TextIOBase raises UnsupportedOperation; the text descriptors read as None.
tx = _io._TextIOBase()
for label, fn in [("read", lambda: tx.read()),
                  ("readline", lambda: tx.readline()),
                  ("write", lambda: tx.write("x")),
                  ("detach", lambda: tx.detach())]:
    try:
        fn()
    except UnsupportedOperation as e:
        print("txt", label, "->", e)
print("txt attrs:", tx.encoding, tx.errors, tx.newlines)

# A raw subclass that supplies readinto drives read and readall through the base.
class RawBytes(_io._RawIOBase):
    def __init__(self, data):
        self._d = data
        self._i = 0
    def readable(self):
        return True
    def readinto(self, b):
        chunk = self._d[self._i:self._i + len(b)]
        b[:len(chunk)] = chunk
        self._i += len(chunk)
        return len(chunk)

rb = RawBytes(b"hello world")
print("raw read(5):", rb.read(5))
print("raw readall:", rb.readall())

# A buffered subclass that supplies read drives readinto through the base.
class BufBytes(_io._BufferedIOBase):
    def read(self, size=-1):
        return b"abcd"[:size]

bb = BufBytes()
buf = bytearray(4)
print("buf readinto:", bb.readinto(buf), bytes(buf))

# io.py's own bases: an ABCMeta subclass inherits the whole chain.
class RawIOBase(_io._RawIOBase, metaclass=abc.ABCMeta):
    __slots__ = ()

class BufferedIOBase(_io._BufferedIOBase, metaclass=abc.ABCMeta):
    __slots__ = ()

class TextIOBase(_io._TextIOBase, metaclass=abc.ABCMeta):
    __slots__ = ()

print([c.__name__ for c in RawIOBase.__mro__])
print([c.__name__ for c in BufferedIOBase.__mro__])
print([c.__name__ for c in TextIOBase.__mro__])
print(isinstance(rb, _io._RawIOBase), isinstance(bb, _io._BufferedIOBase))
