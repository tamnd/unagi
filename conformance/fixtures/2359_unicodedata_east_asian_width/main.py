import unicodedata

# unicodedata.east_asian_width returns one of the six width classes F, H, W, Na, A
# and N, read from the pinned CPython 3.14.6 UCD. The earlier build carried only a
# wide-block heuristic that returned F, W, Na or N, so it dropped the Halfwidth
# (H) and Ambiguous (A) classes and mislabeled the accented Latin, Greek and
# general punctuation the whole terminal-column math depends on. The code points
# are printed as \u escapes (ascii()) so the output has no 0x-prefixed hex.
points = [
    0x41, 0x30, 0x20,            # narrow ASCII
    0x4E2D, 0xD55C, 0x3042, 0x3400,  # wide CJK, Hangul, kana, CJK ext A
    0xFF21, 0xFF10, 0x3000,      # fullwidth letter, digit, ideographic space
    0xFF61, 0xFF71, 0xFFE9, 0x20A9,  # halfwidth forms and the won sign
    0x00E9, 0x00A1, 0x03B1, 0x2010, 0x00B1, 0x2460,  # ambiguous
    0x65, 0x0303,                # narrow base letter and a combining tilde
]
for cp in points:
    print(ascii(chr(cp)), unicodedata.east_asian_width(chr(cp)))

# The unassigned code points default to Neutral, and a lone surrogate is Neutral
# too. A control character is Neutral.
for cp in [0xFFFF, 0x0001, 0x10FFFF, 0xDC80]:
    print(ascii(chr(cp)), unicodedata.east_asian_width(chr(cp)))

# The ucd_3_2_0 accessor answers from the same property.
print("ucd320", unicodedata.ucd_3_2_0.east_asian_width("中"))
