# stringprep exposes the RFC 3454 tables. Most membership tests read
# unicodedata general categories, so this exercises them across the category
# driven tables plus the mapping tables. Lone surrogates (in_table_c5) are
# built with chr() so they stay single code points.
#
# in_table_a1 (unassigned code points) is only probed with points that are
# unassigned in both Unicode 3.2 and the current UCD, since a point assigned
# after 3.2 would depend on the frozen ucd_3_2_0 data.
import stringprep as sp

# A.1 unassigned code points, stable across Unicode versions.
for cp in (0x2065, 0x05EB, 0x0378, 0x0530, 0xE0000):
    print("a1", hex(cp), sp.in_table_a1(chr(cp)))
# Noncharacters are excluded from A.1 even though unassigned.
for cp in (0xFDD0, 0x1FFFE, 0x10FFFF):
    print("a1", hex(cp), sp.in_table_a1(chr(cp)))
print("a1", "A", sp.in_table_a1("A"))

# B.1 commonly mapped to nothing.
for cp in (0x00AD, 0x034F, 0x200B, 0xFEFF, 0x41):
    print("b1", hex(cp), sp.in_table_b1(chr(cp)))

# B.2 / B.3 case folding mappings.
for ch in ("A", "ß", "İ", "I", "Σ"):
    print("b2", repr(ch), repr(sp.map_table_b2(ch)))
    print("b3", repr(ch), repr(sp.map_table_b3(ch)))

# C.1.1 / C.1.2 space characters.
for cp in (0x20, 0x00A0, 0x2028, 0x3000, 0x41):
    print("c11", hex(cp), sp.in_table_c11(chr(cp)))
    print("c12", hex(cp), sp.in_table_c12(chr(cp)))
    print("c11_c12", hex(cp), sp.in_table_c11_c12(chr(cp)))

# C.2.1 / C.2.2 control characters.
for cp in (0x09, 0x7F, 0x0080, 0x2028, 0x41):
    print("c21", hex(cp), sp.in_table_c21(chr(cp)))
    print("c22", hex(cp), sp.in_table_c22(chr(cp)))

# C.3 private use, C.4 noncharacters, C.5 surrogates.
for cp in (0xE000, 0xF0000, 0x41):
    print("c3", hex(cp), sp.in_table_c3(chr(cp)))
for cp in (0xFFFE, 0xFDD0, 0x10FFFF, 0x41):
    print("c4", hex(cp), sp.in_table_c4(chr(cp)))
for cp in (0xD800, 0xDC00, 0xDFFF, 0x41):
    print("c5", hex(cp), sp.in_table_c5(chr(cp)))

# C.6 inappropriate for plain text, C.7 inappropriate for canonical rep.
for cp in (0xFFF9, 0xFFFD, 0x41):
    print("c6", hex(cp), sp.in_table_c6(chr(cp)))
for cp in (0x2FF0, 0x2FFB, 0x41):
    print("c7", hex(cp), sp.in_table_c7(chr(cp)))

# C.8 change display / deprecated, C.9 tagging characters.
for cp in (0x0340, 0x200E, 0x202A, 0x41):
    print("c8", hex(cp), sp.in_table_c8(chr(cp)))
for cp in (0xE0001, 0xE007F, 0x41):
    print("c9", hex(cp), sp.in_table_c9(chr(cp)))

# D.1 bidi R/AL, D.2 bidi L.
for cp in (0x05BE, 0x0627, 0x41):
    print("d1", hex(cp), sp.in_table_d1(chr(cp)))
    print("d2", hex(cp), sp.in_table_d2(chr(cp)))
