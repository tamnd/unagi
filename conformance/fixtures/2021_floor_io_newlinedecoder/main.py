# _io.IncrementalNewlineDecoder wraps another incremental decoder and translates
# newlines while decoding, so a TextIOWrapper reading in universal-newline mode
# turns "\r\n" and "\r" into "\n" and remembers which line endings it has seen.
# It holds a trailing "\r" back between chunks so a "\r\n" split across two
# decode calls still collapses to one "\n", and it exposes the seen endings
# through the newlines property. Unlike the stream classes it subclasses object
# directly, not _IOBase. This floor drives it over a real utf-8 incremental
# decoder on ASCII bytes. This is sub-slice 5h of the _io arc (Spec 2076 stdlib
# S0_io_arc.md); the old io shim has none, so nothing runs in parallel. NB two
# things are left out as orthogonal, pre-existing behaviour: only ASCII bytes
# are fed (unagi's utf-8 incremental decoder does not yet buffer a partial
# multibyte sequence across chunks, a codecs-accelerator gap), and __dict__ is
# not probed (a NewClass instance carries an empty __dict__ where this leaf C
# type has none).
import _io
import codecs

D = _io.IncrementalNewlineDecoder


def dec():
    return codecs.getincrementaldecoder("utf-8")()


# It subclasses object directly, not the IOBase tower.
print([c.__name__ for c in D.__mro__])

# translate on: every ending kind collapses to "\n" and all three are recorded.
d = D(dec(), True)
print(repr(d.decode(b"a\r\nb\rc\nd")), repr(d.newlines))

# a "\r\n" split across two decodes still collapses to one "\n".
d = D(dec(), True)
print(repr(d.decode(b"a\r")), repr(d.newlines))
print(repr(d.decode(b"\nb")), repr(d.newlines))

# translate off leaves the bytes but still records the endings.
d = D(dec(), False)
print(repr(d.decode(b"a\r\nb")), repr(d.newlines))

# each single ending kind on its own.
for data in (b"a\nb", b"a\rb", b"a\r\nb"):
    d = D(dec(), True)
    print(repr(d.decode(data)), repr(d.newlines))

# a pending "\r" is flushed as "\n" on a final decode.
d = D(dec(), True)
print(repr(d.decode(b"a\r")), repr(d.decode(b"", True)), repr(d.newlines))

# getstate folds the pending "\r" into the flag, and setstate restores it.
d = D(dec(), True)
d.decode(b"a\r")
st = d.getstate()
print("state", st)
d.setstate(st)
print("resumed", repr(d.decode(b"\n")))

# reset clears both the pending "\r" and the seen-ending set.
d.reset()
print("post reset", repr(d.newlines))

# an empty decode returns an empty str and records nothing.
d = D(dec(), True)
print(repr(d.decode(b"")), repr(d.newlines))

# a final decode of a whole "\r\n" collapses in one pass.
d = D(dec(), True)
print(repr(d.decode(b"x\r\ny", True)), repr(d.newlines))

# translate and errors accept keyword form.
d = D(dec(), translate=True, errors="strict")
print("kw", repr(d.decode(b"p\r\nq")))

# the wrapped decoder, translate flag and errors name stay hidden.
print(hasattr(d, "decoder"), hasattr(d, "translate"), hasattr(d, "errors"))

# argument errors.
try:
    D()
except TypeError as e:
    print("noargs:", e)
try:
    D(dec())
except TypeError as e:
    print("notrans:", e)
try:
    D(dec(), True, "strict", "extra")
except TypeError as e:
    print("toomany:", e)
