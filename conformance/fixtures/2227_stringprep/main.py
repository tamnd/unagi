import stringprep
import unicodedata

# stringprep binds unicodedata.ucd_3_2_0 as its UCD and asserts the version at
# import; the accessor reports 3.2.0, the Unicode version RFC 3454 is written
# against, matching CPython.
print("imported")
print(unicodedata.ucd_3_2_0.unidata_version)
print(unicodedata.ucd_3_2_0 is not unicodedata)

# Category-driven tables answer for real from the general category Go tracks.
print(stringprep.in_table_a1("A"))            # assigned -> False
print(stringprep.in_table_c11(" "))           # ASCII space -> True
print(stringprep.in_table_c11("A"))           # not space -> False
print(stringprep.in_table_c12(" "))      # line separator (Zl, not Zs) -> False
print(stringprep.in_table_c11_c12(" "))       # Zs -> True
print(stringprep.in_table_c21("\x01"))        # ASCII control -> True
print(stringprep.in_table_c21("A"))           # printable -> False
print(stringprep.in_table_c3(""))       # private use (Co) -> True

# Set-driven tables are hardcoded in stringprep and independent of the UCD.
print(stringprep.in_table_b1("­"))       # soft hyphen -> True
print(stringprep.in_table_b1("A"))            # not in set -> False
print(stringprep.in_table_c6("￹"))       # in c6_set -> True
print(stringprep.in_table_c8("‪"))       # LRE, in c8_set -> True
