class BA(bytearray):
    pass


def show(label, e):
    try:
        v = e()
        print(label, repr(v), type(v).__name__)
    except Exception as ex:
        print(label, "ERR", type(ex).__name__, ex)


# The bytes() and bytearray() constructors take source, encoding and errors as
# position-or-keyword arguments, matching CPython's clinic signature, so a source
# passed by name and a string source with a keyword encoding both build.
show("src-bytes-kw", lambda: bytes(source=b"x"))
show("src-str-enc-kw", lambda: bytes(source="hi", encoding="utf-8"))
show("str-enc-err-kw", lambda: bytes("hi", encoding="utf-8", errors="strict"))
show("src-int-kw", lambda: bytes(source=3))
show("src-iter-kw", lambda: bytes(source=[65, 66]))
show("ba-src-kw", lambda: bytearray(source=b"x"))
show("ba-str-enc-kw", lambda: bytearray(source="hi", encoding="utf-8"))
show("ba-int-kw", lambda: bytearray(source=3))

# Encoding or errors without a string source names whichever was given, and a
# string source without an encoding is the reverse error.
show("enc-only", lambda: bytes(encoding="utf-8"))
show("errors-only", lambda: bytes(errors="strict"))
show("src-bytes-errors", lambda: bytes(source=b"hi", errors="strict"))
show("src-bytes-enc", lambda: bytes(source=b"hi", encoding="utf-8"))
show("enc-errors-nosrc", lambda: bytes(encoding="utf-8", errors="strict"))
show("str-no-enc", lambda: bytes(source="hi"))
show("src-str-errors-noenc", lambda: bytes(source="hi", errors="strict"))

# A slot given twice, an unknown keyword and more than three arguments each raise
# CPython's own message, and the count check outranks the duplicate detection.
show("src-and-pos", lambda: bytes(b"a", source=b"b"))
show("enc-name-pos", lambda: bytes("hi", "utf-8", encoding="utf-8"))
show("bad-kw", lambda: bytes(foo=1))
show("ba-bad-kw", lambda: bytearray(baz=2))
show("err-name-pos", lambda: bytes("hi", "utf-8", "strict", errors="x"))
show("too-many-pos", lambda: bytes("a", "b", "c", "d"))

# The keyword form reaches the constructor through a value reference and a **kwargs
# unpack too, not only the direct call.
f = bytes
show("value-ref", lambda: f(source=b"top"))
show("kwargs-unpack", lambda: bytearray(**{"source": "ab", "encoding": "ascii"}))

# A bytearray subclass fills in __init__, so it inherits the keyword source and
# rebuilds the subclass type.
show("subclass-src-kw", lambda: BA(source=b"x"))
show("subclass-str-enc-kw", lambda: BA(source="hi", encoding="utf-8"))
show("subclass-bad-kw", lambda: BA(nope=1))
