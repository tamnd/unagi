# time.struct_time and time.strftime, the pieces the pure-Python datetime builds
# in _build_struct_time and calls back for its own strftime. struct_time is a
# callable structseq: it takes a nine-tuple, exposes the nine sequence fields plus
# the named-only tm_zone and tm_gmtoff, and behaves as a tuple. strftime renders
# the C directives from those fields. Every value here is a fixed date or literal,
# identical on every host.

import time

tt = (2026, 7, 24, 13, 30, 45, 4, 205, 0)
st = time.struct_time(tt)
print(st)
print(len(st), st[0], st[8], st.tm_year, st.tm_mon, st.tm_isdst)
print(st.tm_zone, st.tm_gmtoff, isinstance(st, tuple))
print(list(st), st[2:5])

# The directive surface, over a fixed Friday.
for f in [
    "%Y-%m-%d %H:%M:%S",
    "%a %A %b %B %p",
    "%y %C %I %j %w %u",
    "%U %W %V %G",
    "%c",
    "%x | %X",
    "100%% done on %e",
]:
    print(f, "->", time.strftime(f, tt))

# An eleven-tuple fills tm_zone and tm_gmtoff, but the repr still shows nine.
u = time.struct_time((2026, 7, 24, 13, 30, 45, 4, 205, 0, "UTC", 0))
print(u.tm_zone, u.tm_gmtoff)
print(u)

# ISO week edges where the week-numbering year differs from the calendar year.
for d in [
    (2021, 1, 1, 0, 0, 0, 4, 1, 0),
    (2024, 12, 30, 0, 0, 0, 0, 365, 0),
    (2023, 1, 1, 0, 0, 0, 6, 1, 0),
]:
    print(time.strftime("%G-W%V %U %W", d))

# datetime drives the same path through strftime and timetuple.
import datetime as dt

print(dt.date(2026, 7, 24).strftime("%Y/%m/%d (%a)"))
print(dt.datetime(2026, 3, 4, 9, 5, 7).strftime("%c"))
print(tuple(dt.date(2026, 7, 24).timetuple()))

# The short-sequence and long-sequence construction errors.
try:
    time.struct_time((1, 2, 3))
except TypeError as e:
    print("short:", e)
try:
    time.struct_time((0,) * 13)
except TypeError as e:
    print("long:", e)
