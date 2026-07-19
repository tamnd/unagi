# codecs is the pure-Python module behind text encoding, standing on the
# _codecs accelerator for the registry and the utf-8/ascii/latin-1 codecs and
# on the encodings package for the cold-path lookup that maps a name to a
# CodecInfo. This drives that whole path: the stateless entry points, the
# registry lookup, the alias resolution, and the error-handler table.
import codecs

# The stateless encode and decode entry points, default and named codecs.
print(codecs.encode("h\xe9llo", "utf-8"))
print(codecs.decode(b"h\xc3\xa9llo", "utf-8"))
print(codecs.encode("hi", "ascii"), codecs.decode(b"hi", "ascii"))
print(codecs.encode("\xff", "latin-1"), codecs.decode(b"\xff", "latin-1"))

# lookup resolves a name to a CodecInfo through the encodings search function,
# folding case and spelling and following the alias table.
for enc in ("utf-8", "utf8", "UTF-8", "ascii", "us-ascii", "latin-1", "iso-8859-1"):
    info = codecs.lookup(enc)
    print(enc, "->", type(info).__name__, info.name)

# The CodecInfo carries callable encode and decode that round-trip through the
# accelerator and report the consumed count.
info = codecs.lookup("utf-8")
print(callable(info.encode), callable(info.decode))
print(info.encode("a\xe9"), info.decode(b"a\xc3\xa9"))

# str.encode and bytes.decode reach the same codecs.
print("world".encode("utf-8"), b"world".decode("ascii"))

# The error-handler table: the standard handlers are named, and a custom one
# round-trips through register_error and lookup_error.
print(codecs.lookup_error("strict").__name__, codecs.lookup_error("ignore").__name__)


def my_handler(exc):
    return ("?", exc.end)


codecs.register_error("my_handler", my_handler)
print(codecs.lookup_error("my_handler") is my_handler)

# An unknown encoding raises LookupError; an unknown handler name raises it too.
try:
    codecs.lookup("no-such-codec")
except LookupError as e:
    print("lookup:", e)
try:
    codecs.lookup_error("no-such-handler")
except LookupError as e:
    print("handler:", e)

# __import__ pulls a module in by name the way the encodings search function
# does; with a fromlist it hands back the named module.
mod = __import__("codecs")
print(mod.__name__)
