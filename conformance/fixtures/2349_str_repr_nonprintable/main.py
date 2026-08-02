# repr escapes every character str.isprintable calls non-printable, the way
# CPython consults Py_UNICODE_ISPRINTABLE: an ASCII control stays a \xNN escape,
# and a non-ASCII non-printable is escaped by its magnitude rather than emitted
# raw. A printable character, ASCII or not, is written as itself.

# One representative from each non-printable category, with the width that picks
# the escape: \xNN up to U+00FF, \uNNNN up to U+FFFF, \UNNNNNNNN above it.
nonprintable = [
    0x00,      # Cc, null
    0x1f,      # Cc, unit separator
    0x7f,      # Cc, delete
    0x80,      # Cc, first C1 control
    0x9f,      # Cc, last C1 control
    0xa0,      # Zs, no-break space
    0xad,      # Cf, soft hyphen
    0x200b,    # Cf, zero width space
    0x2028,    # Zl, line separator
    0x2029,    # Zp, paragraph separator
    0x2060,    # Cf, word joiner
    0xe000,    # Co, first private use
    0xfeff,    # Cf, zero width no-break space
    0x110bd,   # Cf, kaithi number sign (astral, needs \U)
    0xe0001,   # Cf, language tag (astral private-use area)
]
for cp in nonprintable:
    c = chr(cp)
    # repr escapes it, and the char reports itself as not printable, so the two
    # views agree.
    print(cp, repr(c), c.isprintable(), ascii(c))

# A printable character is left raw by repr no matter how high the code point,
# and it round trips through eval of its own repr.
printable = [0x41, 0xa1, 0xe9, 0x263a, 0x1f600]
for cp in printable:
    c = chr(cp)
    print(cp, repr(c), c.isprintable(), repr(c) == "'" + c + "'")

# Mixed strings keep the printable runs raw and escape only the gaps, and the
# quote choice still prefers the single quote.
print(repr("a\x80b\x9fc"))
print(repr("caf\xe9 ​ tail"))
print(repr("\U0001f600 \xad face"))
print(repr("it's a \x85 quote"))

# A lone surrogate is not printable, so repr escapes it as \udcNN.
sur = b"\xed\xb2\x80".decode("utf-8", "surrogatepass")
print("surrogate", repr(sur))
