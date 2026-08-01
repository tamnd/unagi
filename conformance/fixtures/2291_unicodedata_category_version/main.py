# unicodedata.category and unidata_version, pinned from the 3.14 UCD as sorted
# code-point ranges. category returns the two-letter general category (Lu, Ll,
# Nd, Po, Cn and the rest), and unidata_version names the UCD the whole module
# answers from. Pinning both means category agrees with CPython 3.14 including
# the blocks assigned in Unicode 16.0, which an older UCD would call unassigned.
import unicodedata as u

print("version:", u.unidata_version)

# one character from a spread of general categories.
samples = [
    ("A", "upper letter"),
    ("z", "lower letter"),
    ("ǅ", "title letter"),
    ("中", "other letter"),
    ("́", "nonspacing mark"),
    ("7", "decimal digit"),
    ("Ⅻ", "letter number"),
    ("½", "other number"),
    ("(", "open punct"),
    ("!", "other punct"),
    ("+", "math symbol"),
    ("$", "currency"),
    (" ", "space sep"),
    ("\x00", "control"),
    ("\U000f0000", "private use"),
    ("\U0003fffd", "unassigned"),
]
for ch, label in samples:
    print("%-16s %s" % (label, u.category(ch)))

# blocks assigned in Unicode 16.0 report their real category, not Cn.
print("garay digit:", u.category("\U00010D40"))
print("ol onal letter:", u.category("\U0001E5D0"))
print("tulu-tigalari:", u.category("\U00011380"))
