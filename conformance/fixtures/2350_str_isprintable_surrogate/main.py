# str.isprintable reports False for a lone surrogate the way CPython does.
# unagi holds a surrogate in its WTF-8 form, and iterating the raw bytes would
# split it into the replacement character U+FFFD, which is a printable symbol,
# so isprintable has to decode the surrogate as one code point (U+D800..U+DFFF,
# category Cs, not printable) to match. A high and a low lone surrogate both
# count as non-printable, on their own and mixed into otherwise printable text.
low = b"\xed\xb2\x80".decode("utf-8", "surrogatepass")
high = b"\xed\xa0\x80".decode("utf-8", "surrogatepass")
print("low", low.isprintable())
print("high", high.isprintable())
print("mixed low", ("a" + low + "b").isprintable())
print("mixed high", (high + "tail").isprintable())

# The ordinary printable and non-printable cases are unaffected: an empty string
# and plain text are printable, a control, a no-break space or a zero-width space
# is not, and an ASCII space stays printable.
cases = ["", "abc", "caf\xe9", "\U0001f600", chr(0x00), chr(0x1f), chr(0x7f),
         chr(0x80), chr(0xa0), chr(0x200b), chr(0x2028), chr(0x20)]
for c in cases:
    print(repr(c), c.isprintable())

# isprintable stays consistent with repr: repr escapes exactly the characters
# isprintable calls non-printable, so a string is printable when its repr adds
# no \x, \u or \U escape beyond the surrounding quotes.
for c in ["abc", low, "a\x80b", "caf\xe9"]:
    r = repr(c)
    has_escape = "\\x" in r or "\\u" in r or "\\U" in r
    print("consistent", c.isprintable(), not has_escape, c.isprintable() == (not has_escape))
