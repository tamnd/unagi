def show(label, e):
    try:
        print(label, repr(e()))
    except Exception as ex:
        print(label, "ERR", type(ex).__name__, ex)


# A bytes subclass with no __new__ or __init__ of its own inherits the bytes
# constructor, keyword form included, so the source, encoding and errors keyword
# arguments build the payload the way bytes(...) does at the top level.
class MB(bytes):
    pass


show("source-kw", lambda: MB(source=b"hi"))
show("str-encoding", lambda: MB("hi", encoding="utf-8"))
show("str-encoding-errors", lambda: MB("café", encoding="ascii", errors="replace"))
show("encoding-kw-only", lambda: MB(string="hi", encoding="utf-8"))
show("count-positional", lambda: MB(3))
show("empty", lambda: MB())

# The payload is a real bytes value, so it compares equal to the plain bytes and
# still reports the subclass type and its own repr.
m = MB(source=b"hi")
print("value", m == b"hi", type(m).__name__, isinstance(m, bytes), repr(m))

# The base constructor validates the keywords itself, so an unknown keyword is
# the bytes error naming the argument, a source that needs an encoding without
# one is the reverse error, and an encoding on a bytes source is rejected.
show("unknown-kw", lambda: MB(foo=1))
show("str-without-encoding", lambda: MB("hi"))
show("encoding-without-str", lambda: MB(b"hi", encoding="utf-8"))
show("bad-count", lambda: MB(-1))


# A subclass with a custom __init__ still runs the value through the base
# __new__, which sees the same keywords, so a keyword the base rejects fails
# there even though __init__ would accept it.
class MBinit(bytes):
    def __init__(self, *args, **kwargs):
        self.saved = kwargs.get("tag")


show("init-extra-kw", lambda: MBinit(b"hi", tag=1))
show("init-source-kw", lambda: MBinit(source=b"hi"))


# A custom __init__ that names the source parameter receives the same keyword
# the base __new__ used, so the payload and the recorded attribute agree.
class MBtag(bytes):
    def __init__(self, source):
        self.tag = source


t = MBtag(source=b"hi")
print("shared-kw", t, t.tag)


# float and tuple take no keyword arguments, so a subclass rejects them with the
# base message the way the top-level constructors do.
class MF(float):
    pass


class MT(tuple):
    pass


show("float-pos", lambda: MF(1.5))
show("float-kw", lambda: MF(x=1.5))
show("tuple-pos", lambda: MT([1, 2, 3]))
show("tuple-kw", lambda: MT(iterable=[1, 2, 3]))
