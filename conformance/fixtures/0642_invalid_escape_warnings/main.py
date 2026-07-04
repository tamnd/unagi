# CPython 3.14 keeps the backslash of an unknown escape but prints a
# SyntaxWarning at compile time, once per string literal for the first bad
# escape. The generated program has to replay those warnings itself.
print(repr("\q"))

# Only the first invalid escape in a literal warns, so \w here stays quiet.
print(repr("a\wb\qc"))

# Two adjacent literals are two literals, so both \d and \e warn.
print(repr("\d" "\e"))

# \8 and \9 are past the octal range, so they warn too.
print("\8\9")

x = 3
# Each run between f-string fields is its own literal for the warning, so \q
# warns, \w stays quiet, and \e after the field warns again.
print(f"a\qb\w{x}\e")
