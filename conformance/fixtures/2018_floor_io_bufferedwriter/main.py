# _io.BufferedWriter wraps a writable raw stream and buffers writes: bytes pile
# up in a buffer_size buffer and reach the raw only when the buffer would
# overflow, on an explicit flush, or on seek/close/detach. It subclasses
# _BufferedIOBase and inherits read/read1/readinto (which raise), close/closed,
# the context manager and writelines from _IOBase, so this floor exercises the
# write buffering, flush timing, seek/tell, the raw delegation and the inherited
# surface, driving it over a BytesIO as the raw stream. This is sub-slice 5g of
# the _io arc (Spec 2076 stdlib S0_io_arc.md); the old io shim has no
# BufferedWriter, so nothing runs in parallel. NB memoryview is left out: unagi's
# bytes-like coercion does not yet accept it, a pre-existing gap shared with
# BytesIO.write, orthogonal to this slice.
import _io
from _io import UnsupportedOperation

W = _io.BufferedWriter

# It sits on the buffered base, so isinstance and the MRO answer through it.
print([c.__name__ for c in W.__mro__])
raw = _io.BytesIO()
w = W(raw)
print(isinstance(w, _io._BufferedIOBase), isinstance(w, _io._IOBase))

# writes accumulate and reach the raw only when the buffer would overflow.
raw = _io.BytesIO()
w = W(raw, buffer_size=8)
print(w.write(b"abc"), raw.getvalue())
print(w.write(b"defgh"), raw.getvalue())
print(w.write(b"ij"), raw.getvalue())
w.flush()
print(raw.getvalue(), w.tell())
print(w.readable(), w.writable(), w.seekable())

# a single write at least the buffer size goes straight through.
raw = _io.BytesIO()
w = W(raw, buffer_size=4)
print(w.write(b"0123456789"), raw.getvalue())

# seek flushes the buffer then seeks the raw stream.
raw = _io.BytesIO(b"XXXXXXXX")
w = W(raw, buffer_size=16)
w.write(b"ab")
print(w.seek(0), raw.getvalue())
w.write(b"CD")
w.flush()
print(raw.getvalue())

# read side is unsupported on a write-only buffered stream.
for op in ("read", "read1"):
    try:
        getattr(w, op)()
    except UnsupportedOperation as e:
        print(op + ":", type(e).__name__, e)

# writelines comes from the inherited base.
raw = _io.BytesIO()
w = W(raw, buffer_size=4)
w.writelines([b"aa", b"bb", b"cc"])
w.flush()
print(raw.getvalue())

# a bytearray writes like bytes.
raw = _io.BytesIO()
w = W(raw)
print(w.write(bytearray(b"BA")))
w.flush()
print(raw.getvalue())

# write and flush after close raise, and closed/close delegate to the raw.
raw = _io.BytesIO()
w = W(raw)
w.write(b"tail")
print(w.closed)
w.close()
print(w.closed, raw.closed)
try:
    w.write(b"x")
except ValueError as e:
    print("write:", e)
try:
    w.flush()
except ValueError as e:
    print("flush:", e)

# non-bytes input raises TypeError.
w = W(_io.BytesIO())
try:
    w.write("str")
except TypeError as e:
    print("nonbytes:", e)

# a non-writable raw is rejected at construction.
class RO(_io._RawIOBase):
    def readable(self):
        return True
    def writable(self):
        return False
try:
    W(RO())
except UnsupportedOperation as e:
    print("rowraw:", e)

# buffer_size must be strictly positive.
try:
    W(_io.BytesIO(), 0)
except ValueError as e:
    print("bufsize:", e)

# detach flushes and hands back the raw stream, then buffered ops raise.
raw = _io.BytesIO()
w = W(raw)
w.write(b"zz")
print(w.detach() is raw, raw.getvalue())
try:
    w.write(b"more")
except ValueError as e:
    print("detached:", e)

# the raw property exposes the wrapped stream and the context manager closes it.
raw = _io.BytesIO()
w = W(raw)
print(w.raw is raw)
with W(_io.BytesIO()) as f:
    f.write(b"ctx")
print(f.closed)
print(type(f).__name__)
