# Pattern.sub and subn, the substitution surface over the _sre engine. sub walks
# the subject for every non-overlapping match, copies the text between matches
# through, and splices a replacement in its place; subn returns the same string
# paired with the count. A string replacement is a template (\1, \g<n>, \g<name>,
# \g<0>, and the standard escapes), a callable replacement runs per match.

import re

# A literal replacement drops in at every match.
print(re.sub(r"\d", "#", "a1b2c3"))
print(re.sub(r"ab", "X", "abcabc"))

# Group references reorder or repeat the captured text.
print(re.sub(r"(\w)(\d)", r"\2\1", "a1b2"))
print(re.sub(r"(?P<x>\d)", r"\g<x>!", "a1b2"))
print(re.sub(r"(\d)", r"\g<0>\g<0>", "a1"))

# count caps the substitutions, 0 meaning every match.
print(re.sub(r"a", "X", "aaaa", count=2))
print(re.sub(r"a", "X", "aaaa", count=0))

# subn returns the (string, count) pair.
print(re.subn(r"\d", "#", "a1b2c3"))
print(re.subn(r"z", "#", "abc"))

# An unmatched group expands to the empty string.
print(re.sub(r"(a)|(b)", r"\1\2", "ab"))

# Empty matches are replaced and advance the scan by one.
print(re.sub(r"", "-", "ab"))
print(re.sub(r"x*", "-", "abc"))

# A callable replacement runs per Match and returns the replacement text.
print(re.sub(r"\d", lambda m: "<" + m.group(0) + ">", "a1b2"))
print(re.subn(r"\w", lambda m: m.group().upper(), "ab"))

# escape sequences in a template resolve the CPython way.
print(re.sub(r"a", r"\n", "a"))
print(re.sub(r"a", r"\\", "a"))

# A bytes pattern substitutes over a bytes subject with a bytes template.
print(re.sub(rb"\d", rb"#", b"a1b2"))
print(re.subn(rb"(\w)", rb"[\1]", b"xy"))
print(re.sub(rb"\d", lambda m: b"<" + m.group() + b">", b"a1"))
