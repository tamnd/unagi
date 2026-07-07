# io.StringIO and io.BytesIO are the in-memory streams. They are C in CPython,
# so the runtime provides them in Go behind the io import: a growable buffer
# and a read/write cursor, and each is its own context manager.
import io

s = io.StringIO()
print("wrote:", s.write("hello"))
s.write(" world")
print("value:", s.getvalue())
print("tell:", s.tell())
s.seek(0)
print("read5:", s.read(5))
print("rest:", s.read())
s.seek(0)
print("line:", repr(s.readline()))
print("closed before:", s.closed)
s.close()
print("closed after:", s.closed)
try:
    s.write("x")
except ValueError as e:
    print("write closed:", e)

# An initial value and overwrite from a seek position.
s2 = io.StringIO("hello")
s2.seek(1)
s2.write("X")
print("overwrite:", s2.getvalue())

# Line iteration consumes the stream.
s3 = io.StringIO("a\nbb\nccc")
print("lines:", [ln for ln in s3])

# The with-block enters to the stream and closes it on the way out.
with io.StringIO() as f:
    f.write("in ctx")
    print("ctx value:", f.getvalue())
print("ctx closed:", f.closed)

# Wrong write type.
try:
    io.StringIO().write(b"x")
except TypeError as e:
    print("str TE:", e)

# BytesIO mirrors the surface over bytes.
b = io.BytesIO()
print("b wrote:", b.write(b"hi"))
b.write(b" there")
print("b value:", b.getvalue())
b.seek(0)
print("b read2:", b.read(2))
print("b rest:", b.read())
b2 = io.BytesIO(b"seed")
print("b init:", b2.getvalue())
try:
    io.BytesIO().write("x")
except TypeError as e:
    print("bytes TE:", e)

# StringIO must reject a non-str initial value.
try:
    io.StringIO(5)
except TypeError as e:
    print("init TE:", e)
