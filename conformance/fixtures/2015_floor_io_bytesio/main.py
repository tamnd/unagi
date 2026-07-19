# _io.BytesIO is the first concrete stream: an in-memory binary buffer with a
# read/write cursor. It subclasses _BufferedIOBase and inherits the
# close/closed, context-manager, iterator and writelines surface from _IOBase,
# so this floor exercises the buffer-specific methods plus the inherited ones.
# This is sub-slice 5d of the _io arc (Spec 2076 stdlib S0_io_arc.md); the old
# io.BytesIO shim stays in place until the flip.
import _io
from _io import UnsupportedOperation

B = _io.BytesIO

# It sits on the buffered base, so isinstance and the MRO answer through it.
print([c.__name__ for c in B.__mro__])
b = B(b"hello world")
print(isinstance(b, _io._BufferedIOBase), isinstance(b, _io._IOBase))
# A fresh instance carries an empty dict but still takes arbitrary attributes.
print(sorted(b.__dict__))
b.tag = 1
print(b.tag)

# Reads advance the cursor; a negative or None size reads the remainder.
print(b.read(5), b.tell())
print(b.read1(3))
print(b.read(), b.read(-1))
b.seek(0)
print(b.read(None))
print(b.getvalue())

# readinto and readinto1 fill a buffer and report the count.
b.seek(0)
into = bytearray(4)
print(b.readinto(into), bytes(into))
b.seek(0)
into1 = bytearray(3)
print(b.readinto1(into1), bytes(into1))

# Writes overwrite at the cursor and extend past the end; a seek beyond the end
# zero-pads the gap.
w = B()
print(w.write(b"abc"))
w.seek(5)
print(w.write(b"Z"), w.getvalue())

# writelines is inherited from _IOBase and drives write.
wl = B()
wl.writelines([b"a", b"bc", b"d"])
print(wl.getvalue())

# seek with each whence, and reading past the end.
s = B(b"abcdef")
print(s.seek(2), s.seek(1, 1), s.seek(-2, 2))
s.seek(100)
print(s.read(), s.tell())

# truncate shrinks to the cursor or to a size, leaving the cursor put; a size
# past the end leaves the buffer.
t = B(b"abcdef")
t.seek(3)
print(t.truncate(), t.getvalue(), t.tell())
t2 = B(b"ab")
print(t2.truncate(5), t2.getvalue())

# The predicates report true, and unsupported operations still raise.
print(b.readable(), b.writable(), b.seekable())
try:
    b.fileno()
except UnsupportedOperation as e:
    print("fileno ->", e)

# Iteration is inherited: split on the newline byte.
it = B(b"a\nb\nc")
print(list(it))

# Context manager closes on exit; a closed stream raises on every operation.
with B(b"z") as cm:
    print("cm read:", cm.read())
print("closed after with:", cm.closed)

c = B(b"q")
c.close()
print("closed:", c.closed)
for m in ["read", "write", "tell", "readable", "getvalue"]:
    try:
        getattr(c, m)(b"") if m == "write" else getattr(c, m)()
    except ValueError as e:
        print(m, "closed ->", e)

# repr carries the type name.
print(repr(B(b"")).split(" object")[0])
