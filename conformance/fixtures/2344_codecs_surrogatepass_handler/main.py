import codecs


def cps(s):
    return " ".join(str(ord(c)) for c in s)


h = codecs.lookup_error("surrogatepass")
print("lookup", bool(h))

# direct encode: a surrogate passes through as each utf codec's raw bytes
print("== direct encode ==")
for enc in ["utf-8", "utf-16-le", "utf-16-be", "utf-32-le", "utf-32-be", "utf-16", "utf-32"]:
    rep, np = h(UnicodeEncodeError(enc, "\ud800", 0, 1, "x"))
    print(enc, rep, np)

# direct decode: a raw surrogate unit decodes back to the code point
print("== direct decode ==")
for enc, data in [
    ("utf-8", b"\xed\xa0\x80"),
    ("utf-16-le", b"\x00\xd8"),
    ("utf-16-be", b"\xd8\x00"),
    ("utf-32-le", b"\x00\xd8\x00\x00"),
    ("utf-32-be", b"\x00\x00\xd8\x00"),
]:
    rep, np = h(UnicodeDecodeError(enc, data, 0, 1, "x"))
    print(enc, cps(rep), np)

# re-raise: a non-surrogate, a truncated or malformed unit, and a non-utf codec
print("== reraise ==")
cases = [
    ("nonsurrogate", lambda: h(UnicodeEncodeError("utf-8", "a", 0, 1, "x"))),
    ("truncated", lambda: h(UnicodeDecodeError("utf-8", b"\xed\xa0", 0, 1, "x"))),
    ("malformed", lambda: h(UnicodeDecodeError("utf-8", b"\xed\xa0z", 0, 1, "x"))),
    ("nonutf-enc", lambda: h(UnicodeEncodeError("latin-1", "\ud800", 0, 1, "x"))),
]
for label, fn in cases:
    try:
        fn()
        print(label, "NO RAISE")
    except (UnicodeEncodeError, UnicodeDecodeError):
        print(label, "raised")

# inline round trip through the utf-8 codec
print("== inline ==")
print("abc\ud800def".encode("utf-8", "surrogatepass"))
print(cps(b"abc\xed\xa0\x80def".decode("utf-8", "surrogatepass")))
print("\U00010fff\ud800".encode("utf-8", "surrogatepass"))
print(cps(b"\xf0\x90\xbf\xbf\xed\xa0\x80".decode("utf-8", "surrogatepass")))
