# M3 re capstone: the pattern syntax and the method surface end to end.
import re

# Literals, wildcards, character classes, and their negation and ranges.
print(re.findall(r"[a-z]+", "ab CD ef"))
print(re.findall(r"[^0-9]+", "a1b22c"))
print(re.match(r"a.c", "abc").group())
print(re.match(r"\d\w\s", "1a ").group())

# Quantifiers greedy and lazy, and bounded repetition.
print(re.match(r"a+", "aaaa").group())
print(re.match(r"a*?b", "aaab").group())
print(re.match(r"a{2,3}", "aaaa").group())
print(re.match(r"a{2}", "aaaa").group())
print(re.findall(r"\d+?", "123"))

# Anchors and boundaries.
print(bool(re.match(r"^abc$", "abc")))
print(re.findall(r"\bword\b", "a word here word"))
print(re.findall(r"\Bin\B", "the finish inn"))

# Groups: numbered, named, non-capturing, and backreferences.
m = re.match(r"(\w+)-(?P<num>\d+)", "abc-42")
print(m.group(0), m.group(1), m.group("num"), m.groups(), m.groupdict())
print(re.findall(r"(?:ab)+", "abababc"))
print(bool(re.match(r"(\w)\1", "aa")))
print(bool(re.match(r"(\w)\1", "ab")))
print(re.sub(r"(?P<c>.)\1", r"\g<c>", "aabbcc"))

# Alternation and optional groups.
print(re.findall(r"cat|dog", "cat dog cat"))
print(re.match(r"colou?r", "color").group())

# Lookahead and lookbehind, positive and negative.
print(re.findall(r"\d+(?= dollars)", "5 dollars 6 euros"))
print(re.findall(r"\d+(?! dollars)", "5 dollars 6 euros"))
print(re.findall(r"(?<=\$)\d+", "$5 and 6"))
print(re.findall(r"(?<!\$)\b\d+", "$5 and 6"))

# Flags as an argument and inline.
print(re.findall(r"[a-z]+", "AbC", re.IGNORECASE))
print(re.findall(r"(?i)[a-z]+", "AbC"))
print(re.match(r"^b", "a\nb", re.MULTILINE))
print(bool(re.search(r"^b", "a\nb", re.MULTILINE)))
print(re.match(r"a.b", "a\nb", re.DOTALL).group())

# The method surface: search, fullmatch, split, escape.
print(re.search(r"\d+", "abc123def").span())
print(bool(re.fullmatch(r"\d+", "123")))
print(bool(re.fullmatch(r"\d+", "123a")))
print(re.split(r"\s*,\s*", "a, b ,c"))
print(re.escape("a.b*c+"))

# A compiled pattern reused across calls, and its repr.
p = re.compile(r"(\d+)", re.ASCII)
print(p.pattern, p.groups, p.findall("a1b22"))
print(re.compile(r"x", re.I | re.M))
