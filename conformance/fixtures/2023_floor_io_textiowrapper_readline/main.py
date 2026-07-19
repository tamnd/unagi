# _io.TextIOWrapper.readline reads one line at a time, splitting on the line
# ending its newline mode recognises: universal-newline mode (newline None or "")
# recognises "\r\n", "\r" and "\n", while a specific newline splits on exactly
# that string. An optional size caps the characters returned. Iteration and
# readlines build on readline: __iter__ and __next__ are inherited from _IOBase
# and dispatch to this readline, and readlines collects lines until an optional
# hint is passed, using _IOBase's strict greater-than test (BytesIO's own
# readlines stops at greater-or-equal, so the two differ). This is sub-slice
# 5h-3 of the _io arc (Spec 2076 stdlib S0_io_arc.md); the old io shim has no
# TextIOWrapper, so nothing runs in parallel. codecs is imported so the
# accelerator's internal codec construction resolves.
import _io
import codecs


# readline splits on the ending each newline mode recognises. The input mixes
# "\r\n", lone "\r" and "\n" so every mode returns a different set of lines.
for nl in (None, "", "\n", "\r", "\r\n"):
    b = _io.BytesIO(b"a\r\nb\rc\nd")
    t = _io.TextIOWrapper(b, encoding="utf-8", newline=nl)
    out = []
    while True:
        line = t.readline()
        if not line:
            break
        out.append(line)
    print("readline nl", repr(nl), "->", out)

# a size caps the characters returned; the next readline resumes mid-line.
b = _io.BytesIO(b"hello world\nsecond\n")
t = _io.TextIOWrapper(b, encoding="utf-8")
print("readline(5)", repr(t.readline(5)))
print("readline()", repr(t.readline()))
print("readline(100)", repr(t.readline(100)))

# a read and a readline share the decoded buffer, so a sized read leaves the
# rest of the line for the next readline.
b = _io.BytesIO(b"abc\ndef\n")
t = _io.TextIOWrapper(b, encoding="utf-8")
print("read(2)", repr(t.read(2)))
print("then readline", repr(t.readline()))
print("then readline", repr(t.readline()))
print("then readline", repr(t.readline()))

# a stream with no trailing newline yields its last line without one, then "".
b = _io.BytesIO(b"onlyline")
t = _io.TextIOWrapper(b, encoding="utf-8")
print("no eol", repr(t.readline()), repr(t.readline()))

# an empty stream reads "" straight away.
b = _io.BytesIO(b"")
t = _io.TextIOWrapper(b, encoding="utf-8")
print("empty", repr(t.readline()), t.readlines())

# iteration is inherited and walks the lines through readline.
b = _io.BytesIO(b"x\ny\nz")
t = _io.TextIOWrapper(b, encoding="utf-8")
print("iter", list(t))

# readlines with no hint returns every line.
b = _io.BytesIO(b"aa\nbb\ncc\n")
t = _io.TextIOWrapper(b, encoding="utf-8")
print("readlines", t.readlines())

# readlines stops once the total characters read pass the hint; the strict test
# means a hint equal to a line boundary still reads the next line.
for h in (1, 2, 3, 4, 5, 6, 7):
    b = _io.BytesIO(b"aa\nbb\ncc\n")
    t = _io.TextIOWrapper(b, encoding="utf-8")
    print("readlines hint", h, t.readlines(h))

# readline on a closed stream raises, and a non-integer size is rejected; the
# two line readers word the type error differently.
b = _io.BytesIO(b"hi\n")
t = _io.TextIOWrapper(b, encoding="utf-8")
t.close()
try:
    t.readline()
except ValueError as e:
    print("readline closed:", e)
b = _io.BytesIO(b"hi\n")
t = _io.TextIOWrapper(b, encoding="utf-8")
try:
    t.readline("z")
except TypeError as e:
    print("readline badsize:", e)
try:
    t.readlines("z")
except TypeError as e:
    print("readlines badsize:", e)
