# The re package compiled from the pinned CPython source into the floor, so
# import re runs the real _parser and _compiler over the _sre engine. This
# covers the import and the anchored, searching, and group-reading surface a
# program reaches first: compile and its repr, match, search, fullmatch, the
# group accessors, named groups, the span edges, the IGNORECASE, MULTILINE, and
# DOTALL flags, and re.escape. The scanning and substitution methods land in
# their own slices.

import re

print(re.__name__)
print(type(re).__name__)

# The pattern compiles through _parser and _compiler and reprs as its source.
p = re.compile(r"(\d+)-(\d+)")
print(p)
print(p.pattern, p.groups)

# match anchors at the start, search walks forward, fullmatch spans the whole
# subject.
print(re.match(r"(\d+)-(\d+)", "12-34").groups())
print(re.search(r"[a-z]+", "  Hello World  ").group())
print(re.fullmatch(r"\w+@\w+\.\w+", "user@host.com").group())
print(re.match(r"\d+", "abc"))
print(re.fullmatch(r"\d+", "12a"))

# group() reads the whole match and a numbered group, span/start/end give the
# offsets, and groups() fills an unmatched optional group with None.
m = re.match(r"(a)(b)?(c)", "ac")
print(m.group(0), m.group(1), m.group(3))
print(m.groups())
print(m.span(), m.start(), m.end())

# Named groups read back by name and gather into groupdict.
d = re.match(r"(?P<year>\d{4})-(?P<mon>\d{2})", "2026-07")
print(d.group("year"), d.group("mon"))
print(d.groupdict())
print(d.lastgroup)

# The case, multiline, and dotall flags flow from re into the engine.
print(re.match(r"ab+c", "ABBBC", re.IGNORECASE).group())
print(re.search(r"^bar", "foo\nbar", re.MULTILINE).group())
print(re.match(r"a.c", "a\nc", re.DOTALL).group())
print(re.compile(r"x", re.IGNORECASE | re.DOTALL))

# A string pattern carries the implied UNICODE flag.
print(re.compile(r"x").flags & re.UNICODE == re.UNICODE)

# escape backslash-escapes the regex metacharacters through str.translate.
print(re.escape("a.b*c+(d)"))

# Alternation and the optional quantifier both resolve.
print(re.match(r"colou?r", "color").group(), re.match(r"colou?r", "colour").group())

# A bytes pattern matches a bytes subject.
mb = re.match(rb"(\d+)", b"42x")
print(mb.group(0), mb.group(1))
