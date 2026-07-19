# _io.TextIOWrapper wraps a buffered binary stream and presents a text stream:
# it decodes bytes to str on the way out and encodes str to bytes on the way in,
# translating newlines in both directions. Reading in universal-newline mode
# wraps the codec decoder in an IncrementalNewlineDecoder so "\r\n" and "\r"
# collapse to "\n" and the endings seen show up on the newlines property; the
# write side buffers encoded bytes in the text layer until a flush, a
# write-through, or a line-buffered line ending. This floor drives the core
# surface over a BytesIO on utf-8. This is sub-slice 5h-2 of the _io arc (Spec
# 2076 stdlib S0_io_arc.md); the old io shim has no TextIOWrapper, so nothing
# runs in parallel. codecs is imported so the accelerator's internal
# codecs.getincremental{encoder,decoder} calls resolve. NB readline, iteration
# (5h-3), tell and seek (5h-4), reconfigure and repr are left out as later
# sub-slices; only ASCII plus the single "é" is fed, since unagi's utf-8
# incremental decoder does not yet hold a partial multibyte sequence across a
# decode boundary (a codecs-accelerator gap that a small input never triggers).
import _io
import codecs

T = _io.TextIOWrapper
UnsupportedOperation = _io.UnsupportedOperation


# It subclasses the text half of the IOBase tower.
print([c.__name__ for c in T.__mro__])

# construction reports the encoding, error handler and newline configuration,
# and keeps the wrapped buffer by identity.
b = _io.BytesIO(b"hello")
t = _io.TextIOWrapper(b, encoding="utf-8")
print(t.encoding, t.errors, t.line_buffering, t.write_through, t.newlines)
print("buffer is", t.buffer is b)
print("r/w/s", t.readable(), t.writable(), t.seekable())
print("dict", t.__dict__)
print("isatty", t.isatty())
try:
    t.fileno()
except UnsupportedOperation as e:
    print("fileno:", e)

# a read decodes bytes to text; a sized read counts characters, not bytes, so
# the two-byte "é" is one character.
b = _io.BytesIO("café\nxyz".encode("utf-8"))
t = _io.TextIOWrapper(b, encoding="utf-8")
print("read(2)", repr(t.read(2)))
print("read(3)", repr(t.read(3)))
print("read()", repr(t.read()))

# write returns the number of characters written and encodes to utf-8.
b = _io.BytesIO()
t = _io.TextIOWrapper(b, encoding="utf-8")
print("write", t.write("abcafé"))
t.flush()
print("bytes", b.getvalue())

# write newline modes: None and "" and "\n" leave "\n" alone, "\r" and "\r\n"
# rewrite it (os.linesep is "\n" here).
for nl in (None, "", "\n", "\r", "\r\n"):
    b = _io.BytesIO()
    t = _io.TextIOWrapper(b, encoding="utf-8", newline=nl)
    t.write("a\nb\n")
    t.flush()
    print("write nl", repr(nl), "->", b.getvalue())

# read newline modes: None translates to "\n", "" detects without translating,
# a specific ending neither translates nor records.
for nl in (None, "", "\n", "\r", "\r\n"):
    b = _io.BytesIO(b"a\r\nb\rc\n")
    t = _io.TextIOWrapper(b, encoding="utf-8", newline=nl)
    print("read nl", repr(nl), "->", repr(t.read()), "newlines", repr(t.newlines))

# line buffering holds encoded bytes in the text layer until a line ending
# reaches the wrapped buffer.
b = _io.BytesIO()
t = _io.TextIOWrapper(b, encoding="utf-8", line_buffering=True)
t.write("no newline")
print("lb pending", b.getvalue())
t.write(" now\n")
print("lb flushed", b.getvalue())

# write-through hands each write straight to the wrapped buffer.
b = _io.BytesIO()
t = _io.TextIOWrapper(b, encoding="utf-8", write_through=True)
t.write("x")
print("wt", b.getvalue())

# writelines is inherited and writes each str through write.
b = _io.BytesIO()
t = _io.TextIOWrapper(b, encoding="utf-8")
t.writelines(["one\n", "two\n"])
t.flush()
print("writelines", b.getvalue())

# the context manager closes on exit.
b = _io.BytesIO()
with _io.TextIOWrapper(b, encoding="utf-8") as t:
    t.write("ctx\n")
print("ctx closed", b.closed)

# detach hands back the buffer and disconnects it; encoding still reads, buffer
# reads as None, closed and every buffer operation raise.
b = _io.BytesIO(b"hi")
t = _io.TextIOWrapper(b, encoding="utf-8")
d = t.detach()
print("detach is buffer", d is b, "buffer now", t.buffer, "enc", t.encoding)
try:
    t.closed
except ValueError as e:
    print("closed after detach:", e)
try:
    t.read()
except ValueError as e:
    print("read after detach:", e)

# close closes the wrapped buffer, and reads and writes on a closed stream raise.
b = _io.BytesIO(b"hi")
t = _io.TextIOWrapper(b, encoding="utf-8")
t.close()
print("closed", t.closed, b.closed)
try:
    t.read()
except ValueError as e:
    print("read closed:", e)
try:
    t.write("x")
except ValueError as e:
    print("write closed:", e)

# a non-str write is rejected.
b = _io.BytesIO()
t = _io.TextIOWrapper(b, encoding="utf-8")
try:
    t.write(123)
except TypeError as e:
    print("write type:", e)

# constructor argument errors.
try:
    _io.TextIOWrapper()
except TypeError as e:
    print("noargs:", e)
try:
    _io.TextIOWrapper(_io.BytesIO(), encoding="utf-8", newline="x")
except ValueError as e:
    print("badnl:", e)
try:
    _io.TextIOWrapper(_io.BytesIO(), "utf-8", "strict", None, False, False, "extra")
except TypeError as e:
    print("toomany:", e)
try:
    _io.TextIOWrapper(_io.BytesIO(), bogus=1)
except TypeError as e:
    print("badkw:", e)
try:
    _io.TextIOWrapper(_io.BytesIO(), encoding=5)
except TypeError as e:
    print("enctype:", e)
