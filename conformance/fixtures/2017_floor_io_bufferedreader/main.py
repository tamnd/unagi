# _io.BufferedReader wraps a readable raw stream and serves buffered reads: it
# pulls from the raw in buffer_size blocks and hands the caller bytes out of its
# read-ahead buffer, so read/peek/read1/tell reflect that buffer state. It
# subclasses _BufferedIOBase and inherits write (which raises), close/closed, the
# context manager and iteration from _IOBase, so this floor exercises the
# buffering methods (read/read1/readinto/readinto1/peek/seek/tell), the raw
# delegation and the inherited surface, driving it over a BytesIO as the raw
# stream. This is sub-slice 5g of the _io arc (Spec 2076 stdlib S0_io_arc.md);
# the old io shim has no BufferedReader, so nothing runs in parallel.
import _io
from _io import UnsupportedOperation

B = _io.BufferedReader

# It sits on the buffered base, so isinstance and the MRO answer through it.
print([c.__name__ for c in B.__mro__])
b = B(_io.BytesIO(b"hello world"))
print(isinstance(b, _io._BufferedIOBase), isinstance(b, _io._IOBase))

# read draws through a small buffer; tell accounts for the read-ahead, and peek
# returns buffered bytes without advancing.
b = B(_io.BytesIO(b"0123456789abcdefghij"), buffer_size=8)
print(repr(b.read(3)), b.tell())
print(repr(b.peek()[:4]), b.tell())
print(repr(b.read1(2)))
print(repr(b.read1(100)))
print(repr(b.read()))
print(b.readable(), b.writable(), b.seekable())

# A read larger than the buffer reads whole blocks directly, so no residue is
# left and the next peek fills a fresh block.
b = B(_io.BytesIO(b"0123456789"), buffer_size=4)
print(repr(b.read(7)), b.tell())
print(repr(b.peek()))

# readinto reads exactly the destination length with no residue; readinto1 reads
# at most once, directly into the destination when the buffer is empty.
b = B(_io.BytesIO(b"0123456789"), buffer_size=4)
dst = bytearray(5)
print(b.readinto(dst), repr(bytes(dst)))
dst2 = bytearray(5)
print(b.readinto1(dst2), repr(bytes(dst2)))

# seek discards the buffer; a relative seek accounts for the buffered bytes.
b = B(_io.BytesIO(b"0123456789"), buffer_size=4)
print(repr(b.read(4)))
print(b.seek(5))
print(repr(b.read(2)))
b.seek(-2, 1)
print(repr(b.read(2)))
print(b.seek(-3, 2))
print(repr(b.read()))

# readline and iteration come from the inherited base, working through peek.
b = B(_io.BytesIO(b"aa\nbb\ncc"), buffer_size=4)
print(repr(b.readline()))
print([repr(line) for line in b])

# read past the end returns empty and stays empty.
b = B(_io.BytesIO(b"abc"), buffer_size=2)
print(repr(b.read(10)), repr(b.read(10)), repr(b.read1(5)), repr(b.peek()))

# write is unsupported on a read-only buffered stream.
b = B(_io.BytesIO(b"x"))
try:
    b.write(b"y")
except UnsupportedOperation as e:
    print("write:", e)

# buffer_size must be strictly positive.
try:
    B(_io.BytesIO(b""), 0)
except ValueError as e:
    print("bufsize:", e)

# raw exposes the wrapped stream; closed and close delegate to it.
raw = _io.BytesIO(b"hi")
b = B(raw)
print(b.raw is raw, b.closed)
b.close()
print(b.closed)

# detach hands back the raw stream and disconnects it.
raw = _io.BytesIO(b"data")
b = B(raw)
print(b.detach() is raw)
try:
    b.read()
except ValueError as e:
    print("detached:", e)

# flush is a no-op, and the context manager closes on exit.
b = B(_io.BytesIO(b"zzz"))
print(b.flush())
with B(_io.BytesIO(b"body")) as f:
    print(repr(f.read()))
print(f.closed)
print(type(b).__name__)
