# _io.StringIO is the in-memory text stream: a growable unicode buffer with a
# character cursor. It subclasses _TextIOBase and inherits close/closed, the
# context manager, iteration and writelines from _IOBase, so this floor exercises
# the text methods plus the newline model (write rewrite, universal read decode
# and the newlines property) and the inherited surface. This is sub-slice 5e of
# the _io arc (Spec 2076 stdlib S0_io_arc.md); the old io.StringIO shim stays in
# place until the flip.
import _io
from _io import UnsupportedOperation

S = _io.StringIO

# It sits on the text base, so isinstance and the MRO answer through it.
print([c.__name__ for c in S.__mro__])
s = S("hello\nworld")
print(isinstance(s, _io._TextIOBase), isinstance(s, _io._IOBase))
# A fresh instance carries an empty dict but still takes arbitrary attributes.
print(sorted(s.__dict__))
s.tag = 1
print(s.tag)

# Reads advance the character cursor; a negative or None size reads the rest.
print(s.read(3), s.tell())
print(repr(s.readline()))
print(repr(s.read()))
print(repr(s.read(-1)))
s.seek(0)
print(repr(s.read(None)))
print(s.getvalue())

# Writes overwrite at the cursor and extend past the end; a seek beyond the end
# zero-pads the gap with NUL characters.
w = S()
print(w.write("abc"))
w.seek(5)
print(w.write("Z"), repr(w.getvalue()))

# write returns the original character count even when the newline is rewritten.
crlf = S(newline="\r\n")
print(crlf.write("a\nb\n"), repr(crlf.getvalue()))

# writelines is inherited from _IOBase and drives write.
wl = S()
wl.writelines(["a", "bc", "d"])
print(wl.getvalue())

# The default newline "\n" does no translation and recognizes only "\n".
d = S("a\r\nb\rc\n")
print(repr(d.getvalue()), repr(d.newlines))
print([repr(line) for line in d.readlines()])

# newline=None: universal decode on read, buffer stored already translated.
u = S("a\r\nb\rc\n", newline=None)
print(repr(u.getvalue()), repr(u.newlines))
print([repr(line) for line in u.readlines()])

# newline="": no translation but every ending recognized as a line boundary.
e = S("a\r\nb\rc\n", newline="")
print(repr(e.getvalue()), repr(e.newlines))
print([repr(line) for line in e.readlines()])

# newline="\r": write rewrites "\n" to "\r", readline splits on "\r".
r = S(newline="\r")
r.write("x\ny\nz")
print(repr(r.getvalue()))
r.seek(0)
print([repr(line) for line in r.readlines()])

# A "\r\n" split across two writes is not recombined (each write decodes alone).
sp = S(newline=None)
sp.write("a\r")
sp.write("\nb")
print(repr(sp.getvalue()), repr(sp.newlines))

# readline honours a size cap.
rl = S("abcdef\nghi")
print(repr(rl.readline(4)), repr(rl.readline()))

# seek: absolute, then the relative whences accept only offset 0.
sk = S("abcdef")
sk.read(2)
print(sk.seek(0, 1), sk.seek(0, 2))
try:
    sk.seek(2, 1)
except OSError as ex:
    print("cur-rel ->", ex)
try:
    sk.seek(1, 2)
except OSError as ex:
    print("end-rel ->", ex)
try:
    sk.seek(-1)
except ValueError as ex:
    print("neg seek ->", ex)
try:
    sk.seek(0, 3)
except ValueError as ex:
    print("bad whence ->", ex)

# truncate shrinks to the cursor or a size, leaving the cursor put.
t = S("abcdef")
t.seek(3)
print(t.truncate(), repr(t.getvalue()), t.tell())
t2 = S("abcdef")
print(t2.truncate(2), repr(t2.getvalue()))
try:
    S("x").truncate(-1)
except ValueError as ex:
    print("neg trunc ->", ex)

# The predicates report true; unsupported operations still raise.
print(s.readable(), s.writable(), s.seekable())
print(s.isatty())
for name in ("fileno", "detach"):
    try:
        getattr(S(), name)()
    except UnsupportedOperation as ex:
        print(name, "->", ex)

# encoding/errors read as None on a text buffer.
z = S()
print(z.encoding, z.errors)

# Iteration is inherited: split on the line boundary.
it = S("a\nb\nc")
print([repr(line) for line in it])

# Context manager closes on exit; a closed stream raises on every operation.
with S("z") as cm:
    print("cm read:", repr(cm.read()))
print("closed after with:", cm.closed)

c = S("q")
c.close()
print("closed:", c.closed)
for m in ("read", "write", "tell", "readable", "getvalue"):
    try:
        getattr(c, m)("") if m == "write" else getattr(c, m)()
    except ValueError as ex:
        print(m, "closed ->", ex)

# The initial value must be str.
for bad in (b"x", 123):
    try:
        S(bad)
    except TypeError as ex:
        print("init", type(bad).__name__, "->", ex)

# An illegal newline value is rejected.
try:
    S("x", newline="xx")
except ValueError as ex:
    print("illegal newline ->", ex)

# repr carries the type name.
print(repr(S()).split(" object")[0])
