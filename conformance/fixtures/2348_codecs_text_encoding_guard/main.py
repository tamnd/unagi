import codecs

# The bytes-to-bytes transform codecs, the way test_codecs lists them (bz2_codec
# is left out because this build carries bzip2 decompression only).
bytes_transforms = ["base64_codec", "uu_codec", "quopri_codec", "hex_codec", "zlib_codec"]


# str.encode requires a text codec, so a binary transform is rejected with the
# LookupError that steers the caller to codecs.encode, and the error carries no
# __cause__.
def denied_encode(encoding):
    try:
        "bad input type".encode(encoding)
    except LookupError as e:
        return str(e), e.__cause__
    return "no error", None


for enc in bytes_transforms:
    msg, cause = denied_encode(enc)
    print("enc", enc, msg, "cause", cause)

# bytes.decode and bytearray.decode reject a binary transform the same way, on
# data the codec would otherwise accept.
for enc in bytes_transforms:
    data = codecs.encode(b"encode first to meet any format restrictions", enc)
    try:
        data.decode(enc)
    except LookupError as e:
        print("dec bytes", enc, str(e))
    try:
        bytearray(data).decode(enc)
    except LookupError as e:
        print("dec bytearray", enc, str(e))

# rot_13 is a str-to-str codec, so it is not a text encoding either, and the
# error names codecs.encode on the encode side and codecs.decode on the decode
# side.
try:
    "just an example message".encode("rot_13")
except LookupError as e:
    print("rot13 enc", str(e))
for bad in (b"immutable", bytearray(b"mutable")):
    try:
        bad.decode("rot_13")
    except LookupError as e:
        print("rot13 dec", str(e), e.__cause__)

# The str and bytes constructors go through the same guard.
try:
    bytes("bad", "base64_codec")
except LookupError as e:
    print("bytes(str,enc)", str(e))
try:
    bytearray("bad", "hex_codec")
except LookupError as e:
    print("bytearray(str,enc)", str(e))
try:
    str(b"aGVsbG8=\n", "base64_codec")
except LookupError as e:
    print("str(bytes,enc)", str(e))

# codecs.encode and codecs.decode do not apply the guard: they dispatch straight
# to the codec, so a bytes round trip works and a str given to a bytes codec
# raises the codec's own TypeError rather than the LookupError.
print("codecs hex", codecs.encode(b"abc", "hex_codec"), codecs.decode(b"616263", "hex_codec"))
print("codecs b64", codecs.encode(b"hi", "base64_codec"), codecs.decode(b"aGk=\n", "base64_codec"))
try:
    codecs.encode("str not bytes", "base64_codec")
except TypeError:
    print("codecs.encode str TypeError")

# an alias of a binary transform reports the alias name in the message
try:
    "x".encode("zlib")
except LookupError as e:
    print("alias", str(e))

# the generic encode/decode surface still round trips through the transform codecs
for enc in bytes_transforms:
    b, size = codecs.getencoder(enc)(bytes(range(256)))
    back, _ = codecs.getdecoder(enc)(b)
    print("roundtrip", enc, size, back == bytes(range(256)))

# ordinary text codecs are unaffected by the guard
print("utf8", "héllo".encode("utf-8"))
print("utf16le", "hi".encode("utf-16-le"))
print("latin1", "café".encode("latin-1"))
print("ascii dec", b"abc".decode("ascii"))
print("charmap", "abc".encode("charmap"))
# an unknown codec still reports the unknown-encoding error, not the guard error
try:
    "x".encode("no_such_codec_xyz")
except LookupError as e:
    print("unknown", str(e))
