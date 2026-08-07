def show(label, e):
    try:
        print(label, repr(e()))
    except Exception as ex:
        print(label, "ERR", type(ex).__name__, ex)


# The keyword forms of the str constructor decode a bytes-like object the way
# the positional str(bytes, encoding) form does, with encoding and errors each
# fillable by name or position.
show("encoding-kw", lambda: str(b"hi", encoding="utf-8"))
show("object-kw", lambda: str(object=b"hi", encoding="utf-8"))
show("errors-kw", lambda: str(b"\xff", encoding="ascii", errors="replace"))
show("errors-only", lambda: str(b"\xff", errors="replace"))
show("encoding-pos-errors-kw", lambda: str(b"\xff", "ascii", errors="replace"))
show("all-kw", lambda: str(object=b"hi", encoding="utf-8", errors="strict"))
show("bytearray", lambda: str(bytearray(b"hey"), encoding="ascii"))
show("utf8-multibyte", lambda: str(b"caf\xc3\xa9", encoding="utf-8"))
show("latin1", lambda: str(b"\xe9", encoding="latin-1"))
show("ignore", lambda: str(b"a\xffb", encoding="ascii", errors="ignore"))

# A missing object still decodes to the empty string, but the encoding and
# errors arguments keep their type checks.
show("encoding-no-object", lambda: str(encoding="utf-8"))
show("errors-no-object", lambda: str(errors="replace"))
show("encoding-int", lambda: str(encoding=5))
show("errors-int", lambda: str(b"hi", encoding="utf-8", errors=5))

# A str source cannot be decoded, and a non-bytes-like source with an encoding
# is rejected the way CPython's constructor spells it.
show("decode-str", lambda: str("hi", encoding="utf-8"))
show("decode-int", lambda: str(5, encoding="utf-8"))

# The binding errors: the count check outranks a name-and-position clash, and a
# slot filled by both name and position names the one-based position.
show("dup-object", lambda: str(b"hi", object=b"yo"))
show("dup-encoding", lambda: str(b"hi", "utf-8", encoding="ascii"))
show("dup-errors-count", lambda: str(b"\xff", "ascii", "replace", errors="ignore"))
show("unknown-kw", lambda: str(b"hi", foo=1))

# A bad codec is the lookup error, and an undecodable byte under strict errors
# raises the decode error unchanged.
show("bad-codec", lambda: str(b"hi", encoding="not-a-codec"))
show("strict-fail", lambda: str(b"\xff", encoding="ascii"))

# With no encoding or errors the keyword object form is still the plain str()
# conversion, so a non-bytes object stringifies rather than decoding.
show("object-only", lambda: str(object=5))
show("object-none", lambda: str(object=None))
