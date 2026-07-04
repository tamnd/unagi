# The full backslash-escape set the lexer decodes: simple control escapes,
# one-to-three-digit octal, two-hex \x, and \u/\U unicode.
s = "tab\tnl\nret\rquote\"back\\slash"
print(len(s), s.count("\t"), s.count("\n"))

print(repr("\a\b\f\v"))
print([ord(c) for c in "\a\b\f\v"])

# Octal, at or below \377 so no 3.14 invalid-octal warning fires.
print("\101\102\103", ord("\7"), ord("\40"), ord("\377"))

# A short octal run stops at the first non-octal digit.
print(repr("\0" + "8"), ord("\101"[0]))

# Two-hex \x and four/eight-hex \u/\U land the right code points.
print("\x41\x42", ord("€"), ord("\U0001f600"))
print("caf\xe9", "中文")
print(len("\U0001f600"), ascii("€"))
