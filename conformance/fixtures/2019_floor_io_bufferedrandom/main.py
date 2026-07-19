# _io.BufferedRandom wraps a seekable raw stream and buffers both reads and
# writes around one logical position: it reads ahead like BufferedReader and
# buffers writes like BufferedWriter, resolving the other buffer when the mode
# switches (a read flushes pending writes, a write seeks the raw back over the
# read-ahead and drops it). It subclasses _BufferedIOBase and inherits
# close/closed, the context manager and iteration from _IOBase, so this floor
# exercises the read side, the write side and the read/write interleaving over a
# BytesIO raw. This is sub-slice 5g of the _io arc (Spec 2076 stdlib
# S0_io_arc.md); the old io shim has no BufferedRandom, so nothing runs in
# parallel. NB memoryview is left out: unagi's bytes-like coercion does not yet
# accept it, a pre-existing gap shared with BytesIO.write, orthogonal here.
import _io
from _io import UnsupportedOperation

R = _io.BufferedRandom

# It sits on the buffered base, so isinstance and the MRO answer through it.
print([c.__name__ for c in R.__mro__])
b = R(_io.BytesIO(b"0123456789"), buffer_size=4)
print(isinstance(b, _io._BufferedIOBase), isinstance(b, _io._IOBase))
print(b.readable(), b.writable(), b.seekable())

# read then write: the write overwrites at the logical position.
b = R(_io.BytesIO(b"0123456789"), buffer_size=4)
print(repr(b.read(3)), b.tell())
print(b.write(b"XY"), b.tell())
b.flush()
print(b.raw.getvalue())

# seek then read reflects the write.
print(b.seek(0))
print(repr(b.read(4)))

# write then read flushes the write buffer and reads on from there.
raw = _io.BytesIO(b"ABCDEFGH")
b = R(raw, buffer_size=4)
print(b.write(b"12"))
print(repr(b.read(3)), b.tell())
b.flush()
print(raw.getvalue())

# read, write, read round trip.
raw = _io.BytesIO(b"0123456789")
b = R(raw, buffer_size=4)
print(repr(b.read(2)))
b.write(b"AB")
print(repr(b.read(2)))
b.flush()
print(raw.getvalue())

# writes buffer until an explicit flush.
raw = _io.BytesIO(b"0000000000")
b = R(raw, buffer_size=8)
b.write(b"ab")
print(raw.getvalue(), b.tell())
b.write(b"cd")
print(raw.getvalue())
b.flush()
print(raw.getvalue())

# a write larger than the buffer goes straight through.
raw = _io.BytesIO(b"0" * 20)
b = R(raw, buffer_size=4)
print(b.write(b"ABCDEF"), raw.getvalue())

# the read side: peek, read1, readinto.
b = R(_io.BytesIO(b"hello"), buffer_size=8)
print(repr(b.peek()[:5]), repr(b.read(2)))
b = R(_io.BytesIO(b"world!!"), buffer_size=4)
print(repr(b.read1(2)))
dst = bytearray(3)
print(b.readinto(dst), repr(bytes(dst)))

# truncate at the logical position.
raw = _io.BytesIO(b"0123456789")
b = R(raw, buffer_size=4)
b.seek(4)
print(b.truncate())
b.flush()
print(raw.getvalue())

# read to EOF returns empty and stays empty.
b = R(_io.BytesIO(b"abc"), buffer_size=2)
print(repr(b.read()), repr(b.read()))

# a bytearray writes like bytes.
raw = _io.BytesIO()
b = R(raw)
print(b.write(bytearray(b"BA")))
b.flush()
print(raw.getvalue())

# write after close raises, and close/closed delegate to the raw.
raw = _io.BytesIO()
b = R(raw)
b.write(b"tail")
b.close()
print(raw.closed, b.closed)
try:
    b.write(b"x")
except ValueError as e:
    print("wclosed:", e)

# non-bytes input raises TypeError.
b = R(_io.BytesIO())
try:
    b.write("str")
except TypeError as e:
    print("nonbytes:", e)

# a non-seekable raw is rejected at construction.
class NoSeek(_io._RawIOBase):
    def readable(self):
        return True
    def writable(self):
        return True
    def seekable(self):
        return False
try:
    R(NoSeek())
except UnsupportedOperation as e:
    print("noseek:", e)

# buffer_size must be strictly positive.
try:
    R(_io.BytesIO(), 0)
except ValueError as e:
    print("bufsize:", e)

# detach flushes and hands back the raw stream, then buffered ops raise.
raw = _io.BytesIO()
b = R(raw)
b.write(b"zz")
print(b.detach() is raw, raw.getvalue())
try:
    b.read()
except ValueError as e:
    print("detached:", e)

# the context manager closes on exit.
with R(_io.BytesIO(b"body")) as f:
    print(repr(f.read()))
print(f.closed)
print(type(f).__name__)
