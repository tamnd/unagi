# unicodedata.bidirectional, the Bidi_Class property pinned from the 3.14 UCD as
# sorted code-point ranges. Each script and character type reports its strong or
# weak direction class (L, R, AL, EN, AN, NSM, WS, ON and the rest), which is what
# stringprep's bidi checks and any bidi-aware text layout read. A code point with
# no bidi class answers "".
import unicodedata as u

samples = [
    ("A", "latin"),
    ("z", "latin"),
    ("א", "hebrew"),
    ("ا", "arabic letter"),
    ("٠", "arabic-indic zero"),
    ("5", "ascii digit"),
    ("²", "superscript two"),
    ("́", "combining acute"),
    (" ", "space"),
    ("\t", "tab"),
    ("(", "open paren"),
    ("+", "plus"),
    (",", "comma"),
    ("中", "cjk"),
    ("⁦", "lri"),
    ("\U0003fffd", "unassigned"),
]
for ch, label in samples:
    print("%-18s %r" % (label, u.bidirectional(ch)))

# The property partitions the whole BMP printable-ASCII block the way CPython
# does: letters are L, digits EN, and the punctuation splits across ES/ET/CS/ON.
print("ascii:", "".join(u.bidirectional(chr(c))[:1] or "-" for c in range(0x20, 0x7F)))
