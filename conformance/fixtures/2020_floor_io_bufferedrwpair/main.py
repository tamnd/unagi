# _io.BufferedRWPair pairs a reader over one raw stream with a writer over
# another, so it buffers reads and writes over a pair of one-directional raws
# (a pipe or socket) that could not be one seekable file. It subclasses
# _BufferedIOBase, delegating the read side to an internal BufferedReader and the
# write side to an internal BufferedWriter, and inherits close/closed, the
# context manager and writelines from _IOBase; it is not seekable, so seek/tell,
# detach and fileno stay the inherited methods that raise UnsupportedOperation.
# This floor drives a pair over two BytesIO raws. This is sub-slice 5g of the
# _io arc (Spec 2076 stdlib S0_io_arc.md); the old io shim has no BufferedRWPair,
# so nothing runs in parallel. NB two orthogonal, pre-existing gaps kept out of
# scope here: memoryview is left out of writes (unagi's bytes-like coercion does
# not accept it, shared with BytesIO.write), and isatty after close is not probed
# (the const-False _IOBase predicate does not consult the closed flag, shared
# with BytesIO.isatty), both from earlier slices and orthogonal to this pair.
import _io
from _io import UnsupportedOperation

R = _io.BufferedRWPair

# It sits on the buffered base, so the MRO answers through it.
print([c.__name__ for c in R.__mro__])
r = _io.BytesIO(b"0123456789")
w = _io.BytesIO()
p = R(r, w, 4)
print(isinstance(p, _io._BufferedIOBase), isinstance(p, _io._IOBase))
print(p.readable(), p.writable())

# It is not seekable: seek/tell, detach and fileno stay the inherited raisers.
for op in ("seek", "tell", "detach", "fileno"):
    try:
        getattr(p, op)() if op != "seek" else p.seek(0)
    except UnsupportedOperation as e:
        print(op + ":", e)

# The read side draws from the reader and keeps its read-ahead buffer.
print(repr(p.read(3)), repr(p.peek()[:4]))
print(repr(p.read1(2)))
dst = bytearray(2)
print(p.readinto(dst), repr(bytes(dst)))
dst = bytearray(2)
print(p.readinto1(dst), repr(bytes(dst)))

# The write side buffers until an explicit flush.
print(p.write(b"AB"), w.getvalue())
p.flush()
print(w.getvalue())

# The two sides are independent: reading does not disturb pending writes.
r2 = _io.BytesIO(b"abcdef")
w2 = _io.BytesIO()
p2 = R(r2, w2)
p2.write(b"XY")
print(repr(p2.read(3)))
p2.flush()
print(w2.getvalue())

# writelines feeds the writer through write.
r3 = _io.BytesIO()
w3 = _io.BytesIO()
p3 = R(r3, w3)
p3.writelines([b"a", b"bc"])
p3.flush()
print(w3.getvalue())

# isatty reports a terminal when either side is one.
class Atty(_io._RawIOBase):
    def readable(self):
        return True
    def writable(self):
        return True
    def isatty(self):
        return True
print(R(_io.BytesIO(), Atty()).isatty(), R(Atty(), _io.BytesIO()).isatty())

# A bytearray writes like bytes.
r4 = _io.BytesIO()
w4 = _io.BytesIO()
p4 = R(r4, w4)
print(p4.write(bytearray(b"BA")))
p4.flush()
print(w4.getvalue())

# close closes both raws and closed reports the writer.
r5 = _io.BytesIO(b"data")
w5 = _io.BytesIO()
p5 = R(r5, w5)
p5.write(b"xy")
p5.close()
print(p5.closed, r5.closed, w5.closed)

# After close the read and write ops raise with the C _io messages.
for op, fn in (
    ("read", lambda: p5.read()),
    ("read1", lambda: p5.read1(2)),
    ("readinto", lambda: p5.readinto(bytearray(2))),
    ("readinto1", lambda: p5.readinto1(bytearray(2))),
    ("peek", lambda: p5.peek()),
    ("write", lambda: p5.write(b"z")),
    ("flush", lambda: p5.flush()),
):
    try:
        fn()
    except ValueError as e:
        print(op + ":", e)

# A non-readable reader and a non-writable writer are both rejected.
class NoRead(_io._RawIOBase):
    def readable(self):
        return False
    def writable(self):
        return True
class NoWrite(_io._RawIOBase):
    def readable(self):
        return True
    def writable(self):
        return False
try:
    R(NoRead(), _io.BytesIO())
except UnsupportedOperation as e:
    print("noread:", e)
try:
    R(_io.BytesIO(), NoWrite())
except UnsupportedOperation as e:
    print("nowrite:", e)

# buffer_size must be strictly positive.
try:
    R(_io.BytesIO(), _io.BytesIO(), 0)
except ValueError as e:
    print("bufsize:", e)

# Fewer than two arguments is a TypeError.
try:
    R(_io.BytesIO())
except TypeError as e:
    print("missing:", e)

# Non-bytes input raises TypeError.
try:
    R(_io.BytesIO(), _io.BytesIO()).write("str")
except TypeError as e:
    print("nonbytes:", e)

# The pair keeps no visible instance state and its slots stay hidden.
p6 = R(_io.BytesIO(), _io.BytesIO())
print(p6.__dict__)

# The context manager closes on exit.
with R(_io.BytesIO(b"body"), _io.BytesIO()) as f:
    print(repr(f.read()))
print(f.closed)
print(type(f).__name__)
