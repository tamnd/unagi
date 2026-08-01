# The unicodedata numeric properties (decimal, digit, numeric) and the
# Bidi_Mirrored flag, pinned from the 3.14 UCD. decimal/digit/numeric are three
# nested properties: every decimal digit has all three, a superscript has a digit
# and numeric value but no decimal, and a fraction has only a numeric value.
# mirrored reports the brackets and relations that flip under a right-to-left run.
import unicodedata as u

# decimal: the ASCII, Arabic-Indic, Devanagari and fullwidth digits all answer
# their value; a character with no decimal value falls to the default.
print("decimal 7:", u.decimal("7"))
print("decimal arabic 5:", u.decimal("٥"))
print("decimal deva 5:", u.decimal("५"))
print("decimal fullwidth 7:", u.decimal("７"))
print("decimal super2 default:", u.decimal("²", -1))

# digit is a superset of decimal: the superscripts and subscripts carry a digit
# value but no decimal value.
print("digit super2:", u.digit("²"))
print("digit sub9:", u.digit("₉"))
print("digit half default:", u.digit("½", -1))

# numeric is the widest: the fractions, Roman numerals and CJK numerals carry a
# numeric value with no digit value.
print("numeric 5:", u.numeric("5"))
print("numeric half:", u.numeric("½"))
print("numeric quarter:", u.numeric("¼"))
print("numeric roman X:", u.numeric("Ⅹ"))
print("numeric cjk wan:", u.numeric("万"))

# a character with no value and no default raises the same errors CPython does.
for fn, name in ((u.decimal, "decimal"), (u.digit, "digit"), (u.numeric, "numeric")):
    try:
        fn("A")
    except ValueError:
        print(name, "A: ValueError")

# mirrored: the brackets, angle quotes and math relations report 1, other
# characters report 0.
mirror = ["(", ")", "[", "]", "{", "}", "<", ">", "«", "»", "∫"]
print("mirrored set:", [u.mirrored(c) for c in mirror])
print("mirrored plain:", [u.mirrored(c) for c in ["A", "5", "中", " "]])
