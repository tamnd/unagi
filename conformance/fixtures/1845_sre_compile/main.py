# _sre is the C accelerator behind the re package. re parses and compiles a
# pattern into the SRE bytecode in pure Python, then hands the finished bytecode
# to _sre.compile, which builds the re.Pattern the matcher runs against. This
# exercises the compiled-pattern object and its readable surface: the source
# pattern, the flag bits, the group count, the name index, and the repr.
import _sre

print(_sre.MAGIC)
print(_sre.CODESIZE)
print(_sre.MAXREPEAT)
print(_sre.MAXGROUPS)

LITERAL = 16
SUCCESS = 1

# A bare literal pattern with no groups.
p = _sre.compile("a", 0, [LITERAL, ord("a"), SUCCESS], 0, {}, (None,))
print(p.pattern)
print(p.flags)
print(p.groups)
print(p.groupindex)
print(repr(p))

# Two capture groups, one named, with the unicode flag a string pattern implies.
p2 = _sre.compile("(?P<x>a)(b)", 32, [LITERAL, 97, SUCCESS], 2, {"x": 1}, (None, "x", None))
print(p2.groups)
print(p2.groupindex)
print(repr(p2))

# A bytes pattern shows the unicode flag when set, since it is not the default.
p3 = _sre.compile(b"a", 2, [LITERAL, 97, SUCCESS], 0, {}, (None,))
print(repr(p3))

# Combined non-default flags list in a fixed order joined with a bar.
p4 = _sre.compile("a", 2 | 8 | 16, [LITERAL, 97, SUCCESS], 0, {}, (None,))
print(repr(p4))

# An attribute the pattern does not carry raises AttributeError.
try:
    p.bogus
except AttributeError as e:
    print(e)
