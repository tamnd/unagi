import codecs


def cps(s):
    return " ".join(str(ord(c)) for c in s)


# decode: undecodable bytes become lone surrogates U+DC80..U+DCFF
print("== decode ==")
print(cps(b"foo\x80bar".decode("utf-8", "surrogateescape")))
print(cps(b"foo\x80bar".decode("ascii", "surrogateescape")))
print(cps(b"\xff\xfe".decode("ascii", "surrogateescape")))
print(cps(b"foo\x80bar".decode("latin-1", "surrogateescape")))
print(cps(b"foo\xa5bar".decode("iso-8859-3", "surrogateescape")))

# encode: those lone surrogates go back to the original bytes
print("== encode ==")
print(cps("foo\udc80bar"))
print("foo\udc80bar".encode("utf-8", "surrogateescape"))
print("foo\udc80bar".encode("ascii", "surrogateescape"))
print("foo\udc80bar".encode("latin-1", "surrogateescape"))
print("foo\udca5bar".encode("iso-8859-3", "surrogateescape"))

# round trip through iso-8859-3, a charmap codec with undefined slots
print("== round trip ==")
data = bytes(range(128, 256))
back = data.decode("iso-8859-3", "surrogateescape").encode("iso-8859-3", "surrogateescape")
print(back == data)

# multibyte codec: an undecodable lead byte in cp932 escapes to a surrogate
print("== multibyte ==")
print(cps(b"a\x81 b".decode("cp932", "surrogateescape")))
print("a\udc81 b".encode("cp932", "surrogateescape"))

# a surrogate that is not in the escape range re-raises on encode
print("== non escape ==")
try:
    "\ud800".encode("utf-8", "surrogateescape")
except UnicodeEncodeError:
    print("encode raised")

# an ASCII byte handed to the decode handler re-raises
print("== decode ascii reraise ==")
h = codecs.lookup_error("surrogateescape")
e = UnicodeDecodeError("ascii", b"a", 0, 1, "bad")
try:
    h(e)
except UnicodeDecodeError:
    print("decode raised")

# direct lookup returns the registered handler
print("== lookup ==")
print(codecs.lookup_error("surrogateescape") is codecs.lookup_error("surrogateescape"))
