import codecs
import binascii

# uu_codec is a bytes-to-bytes transform over binascii.b2a_uu/a2b_uu, reached
# through codecs.encode/decode. It wraps the payload in begin/end lines and
# splits it into 45-byte uuencoded lines.
data = b"Hello uu codec payload 123 with some longer content to span two lines maybe!!"
enc = codecs.encode(data, "uu_codec")
print("enc", enc)
print("rt", codecs.decode(enc, "uu_codec") == data)
print("empty", codecs.encode(b"", "uu_codec"))

# The binascii line helpers directly.
print("b2a", binascii.b2a_uu(b"Cat"))
print("b2a_backtick", binascii.b2a_uu(b"", backtick=True))
print("a2b", binascii.a2b_uu(b"#0V%T"))

# Error paths keep CPython's wording.
try:
    binascii.b2a_uu(b"x" * 46)
except binascii.Error as e:
    print("over45", e)
try:
    binascii.a2b_uu(b"#\x01\x02\x03")
except binascii.Error as e:
    print("illegal", e)
try:
    binascii.a2b_uu(b"#0V%T!!x")
except binascii.Error as e:
    print("trailing", e)
