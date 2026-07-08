# Pattern.split, the scanning split over the _sre engine. split breaks the
# subject at every non-overlapping match and returns the list of pieces. When
# the pattern carries groups their captured text interleaves the pieces, with
# None for a group that did not match (the value split keeps, where sub fills in
# the empty string). maxsplit caps the splits, 0 meaning split at every match,
# and an empty match advances the scan by one so it cannot stall.

import re

# A groupless pattern drops the separators and keeps the pieces between them.
print(re.split(r"\W+", "a b  c"))
print(re.split(r"x", "axbxc"))
print(re.split(r"\d+", "abc"))

# A captured separator interleaves the pieces.
print(re.split(r"(\W+)", "a b c"))
print(re.split(r"(-)", "1-2-3"))
print(re.split(r"(?P<sep>[.,])", "a.b,c"))

# An unmatched group reads as None, not the empty string.
print(re.split(r"(x)(y)?", "axbxyc"))

# maxsplit caps the splits, 0 meaning every match.
print(re.split(r",", "a,b,c", maxsplit=1))
print(re.split(r",", "a,b,c,d", maxsplit=2))
print(re.split(r"-", "1-2-3", maxsplit=0))

# Empty matches split too, advancing the scan by one.
print(re.split(r"", "abc"))
print(re.split(r"x*", "abxc"))

# A bytes pattern splits a bytes subject.
print(re.split(rb"\W+", b"a b  c"))
print(re.split(rb"(,)", b"a,b"))
