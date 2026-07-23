# PEP 750 t-strings, the template string literals annotationlib uses through
# `type(t"")`. A t-string does not format and join like an f-string; it builds a
# string.templatelib.Template holding the static string parts interleaved with
# Interpolation objects, each carrying the evaluated value, the verbatim
# expression source, the conversion, and the evaluated format spec.


# A template with two fields, one plain and one with a conversion and spec. The
# type is Template, the static parts and the interpolation values are tuples,
# and each interpolation reports its four pieces.
t = t"a{1 + 2}b{7!r:>4}c"
print(type(t).__name__)
print(t.strings)
print(t.values)
for i in t.interpolations:
    print(repr(i.value), repr(i.expression), repr(i.conversion), repr(i.format_spec))


# Iterating a template yields the static parts and interpolations in order,
# dropping empty strings; repr shows the whole structure.
print(list(t))
print(repr(t))


# An empty t-string is still a Template, with one empty string part and no
# interpolations, not a plain str.
e = t""
print(e.strings, e.interpolations, e.values)
print(type(e) is type(t))


# The conversion is recorded, never applied: the value stays the raw object.
s = t"{[1, 2]!s}"
print(s.interpolations[0].value, s.interpolations[0].conversion)


# A format spec with its own replacement field is evaluated into the spec
# string, the same way an f-string spec is.
width = 6
v = 42
n = t"{v:{width}}"
print(n.strings, repr(n.interpolations[0].format_spec))


# Adjacent fields put an empty string between them, so the parts still number
# one more than the interpolations.
a = t"{1}{2}"
print(a.strings, a.values)


# Adjacent t-string literals concatenate into one template.
c = t"x" t"{v}y"
print(c.strings, c.values)
