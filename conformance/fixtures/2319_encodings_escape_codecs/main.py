import codecs


def guard(label, fn):
    try:
        print(label, fn())
    except (UnicodeDecodeError, UnicodeEncodeError) as e:
        print(label, "ERR", e)


# unicode_escape encode covers the named escapes, the \x/\u/\U ranges and the
# printable band that passes through raw.
print("enc", codecs.unicode_escape_encode("A B~\n\t\r\\\x00\x7f\x01\x80\xe9中\U0001F600'\""))

# Decode reads the named, octal, hex and \N{} escapes. Unknown escapes pass through
# with their backslash but CPython raises a runtime DeprecationWarning for them, so
# they are exercised in the Go unit test rather than here.
print("dec", codecs.unicode_escape_decode(
    b"A\\n\\t\\r\\\\\\'\\\"\\a\\b\\f\\v\\x41\\u4e2d\\U0001F600\\101\\0\\N{LATIN SMALL LETTER A}"))
print("bslashnl", codecs.unicode_escape_decode(b"a\\\nb"))

# Decode error spans and reasons.
guard("badx", lambda: codecs.unicode_escape_decode(b"\\xZZ"))
guard("xshort", lambda: codecs.unicode_escape_decode(b"\\xA"))
guard("truncu", lambda: codecs.unicode_escape_decode(b"\\u12"))
guard("truncU", lambda: codecs.unicode_escape_decode(b"\\U0001F60"))
guard("Ubig", lambda: codecs.unicode_escape_decode(b"\\UFFFFFFFF"))
guard("Nnobrace", lambda: codecs.unicode_escape_decode(b"\\Nfoo"))
guard("Nunknown", lambda: codecs.unicode_escape_decode(b"\\N{NOPE NAME}"))
guard("Nunterm", lambda: codecs.unicode_escape_decode(b"\\N{LATIN"))
guard("trailbs", lambda: codecs.unicode_escape_decode(b"ab\\"))
print("badxrep", codecs.unicode_escape_decode(b"\\xZZ", "replace"))
print("badxignore", codecs.unicode_escape_decode(b"\\xZZ", "ignore"))

# Non-final buffers hold a truncated trailing escape.
print("nftrail", codecs.unicode_escape_decode(b"ab\\", "strict", False))
print("nfmidu", codecs.unicode_escape_decode(b"ab\\u12", "strict", False))
print("nfmidN", codecs.unicode_escape_decode(b"ab\\N{LAT", "strict", False))

# raw_unicode_escape only touches \u and \U; everything else stays literal.
print("renc", codecs.raw_unicode_escape_encode("A\\é中\U0001F600\n"))
print("rdec", codecs.raw_unicode_escape_decode(b"A\\\\\\u4e2d\\U0001F600\\x41"))
print("r3bs", codecs.raw_unicode_escape_decode(b"\\\\\\u0041"))
guard("rUbig", lambda: codecs.raw_unicode_escape_decode(b"\\UFFFFFFFF"))
guard("rtruncu", lambda: codecs.raw_unicode_escape_decode(b"\\u12"))

# Round-trips through the registered codec names, and the incremental decoder.
sample = "héllo\tworld\n中\U0001F600 \x00\x7f"
print("rt_ue", sample.encode("unicode_escape").decode("unicode_escape") == sample)
print("rt_rue", "héllo中\U0001F600".encode("raw_unicode_escape").decode("raw_unicode_escape") == "héllo中\U0001F600")
dec = codecs.getincrementaldecoder("unicode_escape")()
data = b"ab\\u4e2"
print("inc1", dec.decode(data, False))
print("inc2", dec.decode(b"d!", True))
