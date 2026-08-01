# The UnicodeEncodeError/UnicodeDecodeError the runtime codec paths raise carry
# the structured attributes (encoding, object, start, end, reason), not only a
# preformatted message, so a caller or an error handler can read them off the
# caught exception. The object is the whole input, and the span indexes into it.

# A decode error over a single illegal byte in a CJK codec.
try:
    b"abc\xff".decode("euc_jp")
except UnicodeDecodeError as e:
    print("dec", repr(e.encoding), repr(e.object), e.start, e.end, repr(e.reason))
    print("dec_str", str(e))

# A decode error in a two-byte codec: a lead byte with an illegal trail.
try:
    b"ok\xa1\xff".decode("gb2312")
except UnicodeDecodeError as e:
    print("dec2", repr(e.object), e.start, e.end, repr(e.reason))
    print("dec2_str", str(e))

# An encode error over a code point the codec cannot map: object is the input.
try:
    "abc\U0001F600def".encode("gb2312")
except UnicodeEncodeError as e:
    print("enc", repr(e.encoding), repr(e.object), e.start, e.end, repr(e.reason))
    print("enc_str", str(e))

# The attributes survive a manually driven error handler: register one that
# reads .start/.end/.object off the exception it is handed and returns a
# replacement, then encode through it.
import codecs

seen = []


def handler(exc):
    seen.append((exc.encoding, exc.start, exc.end, exc.object[exc.start]))
    return ("?", exc.end)


codecs.register_error("s2probe", handler)
print("via_handler", "abc\U0001F600def".encode("gb2312", "s2probe"))
print("seen", seen)
