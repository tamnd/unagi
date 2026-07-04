x = 42
w = 6
p = 3
val = 3.14159

# A replacement field inside a format spec, evaluated before the value is
# formatted against the built spec.
print(f"{x:{w}}")
print(f"{x:>{w}}")
print(f"{val:.{p}f}")

# Several fields in one spec, including fill and alignment pulled from names.
fill = "*"
align = ">"
print(f"{x:{fill}{align}{w}}")

n = 255
print(f"{n:#0{w}x}")

# Nested f-strings: an f-string appearing inside another's replacement field,
# with the same quote and with a different quote.
inner = "v"
print(f"{f'{inner}'}")
print(f"{f"{inner}"}")

name = "world"
print(f"{f"hi {name}"}")

# A field whose value itself needs a conversion, then a spec with a field.
label = "pi"
print(f"{label!r:>{w}}")

# Spec fields combined with the self-documenting form.
print(f"{val=:.{p}f}")
