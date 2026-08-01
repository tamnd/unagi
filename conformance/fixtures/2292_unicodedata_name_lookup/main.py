# unicodedata.name and unicodedata.lookup, backed by the pinned 3.14 name
# database. name() returns the canonical name of a character, formatting the
# Hangul syllables and the CJK-style "PREFIX-HEX" ranges from a formula and
# reading the rest from the explicit-name table. lookup() reverses that and also
# resolves name aliases (NULL for a control character, which has no name) and
# named sequences (whose result is more than one character).
import unicodedata as u

names = [
    "A",
    "中",  # CJK unified ideograph, hex >= 4 digits
    "㐀",  # first CJK ext A ideograph
    "\U00020000",  # supplementary CJK, 5 hex digits
    "가",  # first Hangul syllable
    "힣",  # last Hangul syllable
    "\U00010D40",  # Garay Digit Zero, named in Unicode 16.0
]
for ch in names:
    print("name %-8s %s" % (repr(ch), u.name(ch)))

# a control character has no name: default when given, else ValueError.
print("default:", u.name("\x00", "NONE"))
try:
    u.name("\x00")
except ValueError as e:
    print("name error:", e)

lookups = [
    "LATIN CAPITAL LETTER A",
    "CJK UNIFIED IDEOGRAPH-4E2D",
    "HANGUL SYLLABLE GA",
    "HANGUL SYLLABLE HIH",
    "NULL",  # name alias for U+0000
    "BELL",  # name alias
    "LATIN SMALL LETTER GHA",  # name alias for U+01A3
    "LATIN CAPITAL LETTER A WITH MACRON AND GRAVE",  # named sequence, two chars
]
for name in lookups:
    r = u.lookup(name)
    print("lookup %-46s %s" % (name, " ".join("%04X" % ord(c) for c in r)))

# a zero-padded algorithmic name and an unknown name both raise KeyError.
for bad in ["CJK UNIFIED IDEOGRAPH-04E2D", "NO SUCH CHARACTER NAME"]:
    try:
        u.lookup(bad)
    except KeyError as e:
        print("lookup error:", e)
