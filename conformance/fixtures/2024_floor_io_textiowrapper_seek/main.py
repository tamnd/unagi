# _io.TextIOWrapper.tell hands back an opaque cookie that seek can return to. In
# the common case the cookie is just the wrapped buffer's byte position, but once
# characters have been consumed since the last chunk boundary it packs the byte
# position of the nearest safe restart point together with the decoder flags,
# the bytes to feed and characters to skip to reach the current character, and
# whether an end-of-file signal is needed. seek reverses that: it repositions the
# buffer, rebuilds the decoder state and replays the bytes. A cur- or end-relative
# seek is only allowed to the current or final position (cookie 0). This is
# sub-slice 5h-4 of the _io arc (Spec 2076 stdlib S0_io_arc.md); the old io shim
# has no TextIOWrapper, so nothing runs in parallel. codecs is imported so the
# accelerator's internal codec construction resolves.
#
# The tell walk feeds the decoder one byte at a time to find a safe restart
# point, so it can only walk over content the incremental decoder decodes
# byte-by-byte. unagi's utf-8 incremental decoder does not yet hold a partial
# multibyte sequence between decode calls (a codecs-accelerator gap), so the walk
# is driven over ASCII here; the multibyte cases below only exercise the non-walk
# paths (a plain-position tell, seek to the start and seek to the end), which do
# not step through a character.
import _io
import codecs

UnsupportedOperation = _io.UnsupportedOperation


# tell at the start is zero and a sized read advances it by the bytes consumed.
b = _io.BytesIO(b"hello world\nsecond line\n")
t = _io.TextIOWrapper(b, encoding="utf-8")
print("start", t.tell())
print("read(5)", repr(t.read(5)))
c = t.tell()
print("cookie", c, type(c).__name__)
print("read(6)", repr(t.read(6)))
t.seek(c)
print("reread", repr(t.read(6)))
t.seek(0)
print("rewound", repr(t.read(5)))

# a mid-line tell packs a cookie that seek walks back to, over ASCII lines.
b = _io.BytesIO(b"hello\nworld\nthird\n")
t = _io.TextIOWrapper(b, encoding="utf-8")
t.readline()
c = t.tell()
print("mid cookie", c)
print("next line", repr(t.readline()))
t.seek(c)
print("reseek line", repr(t.readline()))

# universal-newline reads translate endings, and tell still round-trips.
b = _io.BytesIO(b"a\r\nb\r\nc\r\n")
t = _io.TextIOWrapper(b, encoding="utf-8")
print("uni l1", repr(t.readline()))
c = t.tell()
print("uni l2", repr(t.readline()))
t.seek(c)
print("uni reread", repr(t.readline()), repr(t.readline()))

# a specific newline mode round-trips its cookie too.
b = _io.BytesIO(b"a\r\nb\r\nc\r\n")
t = _io.TextIOWrapper(b, encoding="utf-8", newline="\r\n")
print("spec l1", repr(t.readline()))
c = t.tell()
t.seek(c)
print("spec reread", repr(t.readline()))

# seek to the end reports the final byte position and reads nothing more.
b = _io.BytesIO(b"one two three")
t = _io.TextIOWrapper(b, encoding="utf-8")
print("end", t.seek(0, 2), repr(t.read()))

# non-walk multibyte paths: a plain-position tell, rewind and seek to end all
# work over the two-byte characters without stepping through them.
b = _io.BytesIO("héllo wörld".encode("utf-8"))
t = _io.TextIOWrapper(b, encoding="utf-8")
print("mb start", t.tell())
print("mb all", repr(t.read()))
print("mb end", t.tell())
t.seek(0)
print("mb rewound", repr(t.read()))
print("mb seekend", t.seek(0, 2))

# seek errors: a nonzero relative seek, a negative position, a bad whence, and
# operating on a closed stream.
b = _io.BytesIO(b"data")
t = _io.TextIOWrapper(b, encoding="utf-8")
try:
    t.seek(5, 1)
except UnsupportedOperation as e:
    print("cur nonzero:", e)
try:
    t.seek(5, 2)
except UnsupportedOperation as e:
    print("end nonzero:", e)
try:
    t.seek(-1)
except ValueError as e:
    print("neg:", e)
try:
    t.seek(0, 3)
except ValueError as e:
    print("whence:", e)
t.close()
try:
    t.seek(0)
except ValueError as e:
    print("closed seek:", e)
try:
    t.tell()
except ValueError as e:
    print("closed tell:", e)
