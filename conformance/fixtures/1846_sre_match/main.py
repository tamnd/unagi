# _sre is the C accelerator behind the re package. re parses and compiles a
# pattern into the SRE bytecode in pure Python, hands the finished bytecode to
# _sre.compile, and the compiled pattern runs match, search, and fullmatch
# against a subject. This drives that surface and the re.Match it produces: the
# matched substrings, the group spans, the named-group lookups, and the
# attributes a Match carries. The bytecode lists here are exactly what the real
# compiler emits for the pattern in the comment, so both engines walk the same
# program.
import _sre

# "abc": a bare three-letter literal, no groups.
abc = _sre.compile(
    "abc", 0,
    [14, 12, 3, 3, 3, 3, 3, 97, 98, 99, 0, 0, 0, 16, 97, 16, 98, 16, 99, 1],
    0, {}, (None,))

m = abc.match("abcdef")
print(m.group())
print(m.group(0))
print(m.span())
print(m.start(), m.end())
print(m.string, m.pos, m.endpos)
print(abc.match("abx"))
print(abc.fullmatch("abc").span())
print(abc.fullmatch("abcd"))
print(abc.search("zzabc").span())
print(abc.search("nope"))

# "(a)(b)": two unnamed capture groups.
ab = _sre.compile(
    "(a)(b)", 0,
    [14, 10, 1, 2, 2, 2, 0, 97, 98, 0, 0, 17, 0, 16, 97, 17, 1, 17, 2, 16, 98,
     17, 3, 1],
    2, {}, (None, None, None))

m = ab.match("ab")
print(m.group())
print(m.group(1), m.group(2))
print(m.group(2, 1))
print(m.groups())
print(m.span(1), m.span(2))
print(m[0], m[1], m[2])
print(m.lastindex, m.lastgroup)
print(m.regs)

# "(a)(?P<second>b)": the second group is named, so lastgroup resolves to it.
named = _sre.compile(
    "(a)(?P<second>b)", 0,
    [14, 10, 1, 2, 2, 2, 0, 97, 98, 0, 0, 17, 0, 16, 97, 17, 1, 17, 2, 16, 98,
     17, 3, 1],
    2, {"second": 2}, (None, None, "second"))

m = named.match("ab")
print(m.group("second"))
print(m["second"])
print(m.groupdict())
print(m.lastindex, m.lastgroup)

# "(a)?b": an optional group that stays unmatched when the subject is just "b".
opt = _sre.compile(
    "(a)?b", 0,
    [14, 4, 0, 1, 2, 23, 9, 0, 1, 17, 0, 16, 97, 17, 1, 18, 16, 98, 1],
    1, {}, (None, None))

m = opt.match("ab")
print(m.group(1), m.groups())
m = opt.match("b")
print(m.group(1))
print(m.groups())
print(m.groups("-"))
print(m.span(1), m.start(1), m.end(1))

# "a+": a greedy repeat with no group.
plus = _sre.compile(
    "a+", 0,
    [14, 4, 0, 1, 4294967295, 24, 6, 1, 4294967295, 16, 97, 1, 1],
    0, {}, (None,))
print(plus.match("aaab").group())
print(plus.match("baaa"))
print(plus.search("baaa").span())

# r"\d+": a digit run through a category set.
digits = _sre.compile(
    "\\d+", 0,
    [14, 4, 0, 1, 4294967295, 24, 9, 1, 4294967295, 13, 4, 8, 10, 0, 1, 1],
    0, {}, (None,))
print(digits.search("abc123def").span())
print(digits.search("abc123def").group())

# "[a-z]+": a lowercase-letter run through a range set, matched from a pos.
lower = _sre.compile(
    "[a-z]+", 0,
    [14, 4, 0, 1, 4294967295, 24, 10, 1, 4294967295, 13, 5, 22, 97, 122, 0, 1, 1],
    0, {}, (None,))
print(lower.match("HELLOworld", 5).group())
print(lower.match("HELLOworld", 5).span())

# A bytes pattern used on a str subject raises, and the reverse.
bpat = _sre.compile(b"a", 0, [16, 97, 1], 0, {}, (None,))
try:
    bpat.match("a")
except TypeError as e:
    print(e)
try:
    abc.match(b"abc")
except TypeError as e:
    print(e)

# An out-of-range group and an unknown name both raise IndexError.
m = ab.match("ab")
try:
    m.group(5)
except IndexError as e:
    print(e)
try:
    m.group("nope")
except IndexError as e:
    print(e)
