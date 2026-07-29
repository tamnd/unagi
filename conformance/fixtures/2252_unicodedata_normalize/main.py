# unicodedata.normalize now runs the real UAX #15 algorithm over the pinned
# CPython 3.14.6 decomposition and composition tables, so all four forms answer
# for non-ASCII text instead of raising. combining() and decomposition() read
# from the same tables. This is the concrete blocker the harness helper modules
# (os_helper -> import_helper) hit, which normalize non-ASCII path names.
import unicodedata as ud

# Canonical decompose and precompose round-trip.
print("NFD e-acute", ud.normalize("NFD", "é") == "é")
print("NFC e-acute", ud.normalize("NFC", "é") == "é")

# Compatibility folding: the fi ligature splits to f i.
print("NFKC fi", ud.normalize("NFKC", "ﬁ") == "fi")
print("NFKD fi", ud.normalize("NFKD", "ﬁ") == "fi")

# Hangul is decomposed and recomposed by arithmetic, no table.
print("NFD han", ud.normalize("NFD", "한") == "한")
print("NFC han", ud.normalize("NFC", "한") == "한")

# Stacked marks are put into canonical order (class 220 before 230).
print("reorder", ud.normalize("NFD", "q̣̇") == "q̣̇")

# U+212B ANGSTROM SIGN is a singleton mapping to U+00C5.
print("angstrom", ud.normalize("NFC", "Å") == "Å")

# is_normalized is the fixed-point check.
print("is NFC", ud.is_normalized("NFC", "é"))
print("not NFC", ud.is_normalized("NFC", "é"))

# combining() and decomposition() read the same pinned tables.
print("combining", ud.combining("́"), ud.combining("a"))
print("decomp acute", ud.decomposition("À"))
print("decomp fi", ud.decomposition("ﬁ"))
print("decomp han", ud.decomposition("각"))

# An invalid form is still a ValueError.
try:
    ud.normalize("NFX", "abc")
except ValueError:
    print("bad form ValueError")
