# The error handler is looked up lazily, only when a character cannot be
# encoded, so surrogateescape and surrogatepass are accepted the way os.fsencode
# and os.fsdecode need. utf-8 encodes every representable code point, so the
# round trip is faithful for ascii and non-ascii alike. Narrow codecs hand an
# out-of-range character to the handler: ignore drops it, replace emits '?', an
# unencodable char under surrogateescape raises, and an unknown handler raises
# LookupError only once a real error reaches it.
print("hello".encode("utf-8", "surrogateescape"))
print("hi".encode("utf-8", "bogus"))
print(b"hi".decode("utf-8", "surrogateescape"))

for p in ["file.txt", "cafe", "café", "日本語.py"]:
    b = p.encode("utf-8", "surrogateescape")
    print(b == p.encode("utf-8"), b.decode("utf-8", "surrogateescape") == p)

print("café".encode("ascii", "ignore"))
print("café".encode("ascii", "replace"))
print(bytes("café", "ascii", "ignore"))

try:
    "café".encode("ascii", "surrogateescape")
except UnicodeEncodeError:
    print("encode-raises")
try:
    "café".encode("ascii", "bogus")
except LookupError:
    print("lookup-raises")
