# Pattern.findall and finditer, the scanning family over the _sre engine. Both
# walk the subject for every non-overlapping match, stepping one past an empty
# match so the scan cannot stall. findall returns the strings (a tuple per match
# when the pattern has several groups, the empty string for a group that did not
# match), finditer returns a callable_iterator of Match objects.

import re

# No group: the whole match per hit.
print(re.findall(r"\d+", "a1b22c333"))
print(re.findall(r"ab", "xabyabz"))

# One group: that group per hit.
print(re.findall(r"(\d)\w", "1a2b3c"))

# Several groups: a tuple per hit, empty string for an unmatched group.
print(re.findall(r"(a)(b)?", "aab"))
print(re.findall(r"(?P<x>\w)(?P<y>\d)", "a1b2c3"))

# Empty matches are counted and advance the scan by one.
print(re.findall(r"a*", "aba"))
print(re.findall(r"", "ab"))
print(re.findall(r"(b)?", "a"))

# A pos and endpos window clips the scan.
print(re.compile(r"\d").findall("1234", 1, 3))

# finditer yields Match objects lazily and its type is callable_iterator.
it = re.finditer(r"\w+", "foo bar baz")
print(type(it).__name__)
print([m.group() for m in it])
print([(m.start(), m.end()) for m in re.finditer(r"o", "foobar")])

# next drives the same cursor a for loop would, and the iterator exhausts.
walk = re.finditer(r"\d", "1a2")
print(next(walk).group(), next(walk).group())
print(next(walk, "done"))
print(list(re.finditer(r"z", "abc")))

# A bytes pattern scans a bytes subject.
print(re.findall(rb"\d", b"a1b2c3"))
