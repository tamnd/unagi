# A raw f-string keeps backslashes literally but still interpolates fields.
x = 42
name = "world"
print(rf"\n{x}\t")
print(fr"C:\path\{name}")

# An unknown escape stays literal and fires no invalid-escape warning.
print(rf"raw\q{x}end")

# The format spec of a raw f-string is also raw.
print(rf"{x:\>6}")

# The prefix is case insensitive in either order.
print(RF"\d{x}")
print(Fr"a\b{name}c")

# Doubled braces still collapse, and conversions still apply.
print(rf"{{lit}}\n{x!r}")

# Triple-quoted raw f-strings work the same way.
print(rf"""line\one
{name}""")
