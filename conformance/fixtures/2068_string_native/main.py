# _string is the C accelerator str.format is built on. string/__init__.py opens
# with `import _string` and drives its Formatter through formatter_parser and
# formatter_field_name_split, with no pure-Python fallback, so the string module
# and str.format cannot work without it.

import _string

# formatter_parser splits a format string into (literal, field, spec, conversion)
# tuples: a doubled brace flushes a literal with a single brace, a field with no
# `:` reads an empty spec, and a trailing literal reads None for the rest.
def show(s):
    print(repr(s), list(_string.formatter_parser(s)))

show("literal only")
show("a{0}b")
show("{name!r:>{width}}")
show("{{escaped}} {x}")
show("{}")
show("{0.attr[key]}")

# A stray brace is the ValueError CPython raises.
try:
    list(_string.formatter_parser("}"))
except ValueError as e:
    print("stray:", e)

# formatter_field_name_split separates the leading argument (int when all digits)
# from the .attr and [key] accessors.
def split(s):
    first, rest = _string.formatter_field_name_split(s)
    print(repr(s), repr(first), list(rest))

split("0")
split("name")
split("obj.attr[key][0]")
split("name[complex key]")

# The string module and str.format now work end to end.
import string

print("digits", string.digits)
fmt = string.Formatter()
print("format", fmt.format("{0} then {name!r:>4}", "a", name="b"))
print("template", string.Template("$who likes $what").substitute(who="I", what="it"))
