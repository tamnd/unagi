# str.encode / bytes.decode reach the utf-16, utf-32, utf-8-sig and utf-7
# codecs the encodings package resolves at runtime, with no `import codecs`.
# The core utf-8/ascii/latin-1 path is unchanged; these are the multibyte and
# BOM-carrying codecs that only work once encodings is pulled into the build.
s = "aα"

for enc in ["utf-16", "utf-16-le", "utf-16-be", "utf-32", "utf-32-le", "utf-32-be", "utf-8-sig", "utf-7"]:
    b = s.encode(enc)
    back = b.decode(enc)
    print(enc, b, back == s)

# A codec named by a variable resolves the same way a literal does.
name = "utf-16"
print("via variable:", s.encode(name))

# utf-16 with a byte-order mark decodes by detecting the BOM.
print("bom le:", b"\xff\xfea\x00".decode("utf-16"))
print("bom be:", b"\xfe\xff\x00a".decode("utf-16"))

# The two-argument bytes and str constructors share the same codec path.
print("bytes ctor:", bytes("hi", "utf-16"))
print("str ctor:", str(b"\xff\xfeh\x00i\x00", "utf-16"))

# A character the target charmap codec cannot hold raises UnicodeEncodeError,
# the codec's own error, not the unknown-encoding LookupError.
try:
    s.encode("cp1252")
except UnicodeEncodeError as e:
    print("cp1252:", type(e).__name__)

# An encoding that truly does not exist still raises LookupError.
try:
    s.encode("no-such-codec")
except LookupError as e:
    print("unknown:", type(e).__name__, e)
