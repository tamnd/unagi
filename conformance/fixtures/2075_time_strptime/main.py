# time.strptime parses a time string into a struct_time, and datetime.strptime
# rides on the same _strptime parser to build a datetime. The C time module
# delegates both to the pure-Python _strptime the floor already carries, so this
# checks the delegation covers the directive surface and the error paths.
# Formats here avoid %Z, since the zone names it parses are host dependent.

import time
import datetime as dt

# The common date and time directives, each producing a struct_time.
print(time.strptime("2026-07-24 13:30:45", "%Y-%m-%d %H:%M:%S"))
print(time.strptime("2000-02-29", "%Y-%m-%d"))
print(time.strptime("2026 205", "%Y %j"))
print(time.strptime("13:30:45", "%H:%M:%S"))
print(time.strptime("Fri, 24 Jul 2026", "%a, %d %b %Y"))
print(time.strptime("July 24, 2026", "%B %d, %Y"))

# The default format matches asctime's shape.
print(time.strptime("Fri Jul 24 13:30:45 2026"))

# datetime.strptime builds a datetime through the same parser.
print(dt.datetime.strptime("2026-07-24T13:30:45", "%Y-%m-%dT%H:%M:%S"))
print(dt.datetime.strptime("07/24/26", "%m/%d/%y").date())
print(dt.datetime.strptime("2026-W30-5", "%Y-W%W-%w"))

# The zone attributes _strptime reads at import: tzname is a two-string tuple,
# daylight is a flag, and timezone and altzone are integers. The values are
# host dependent, so only their shapes are checked.
print(isinstance(time.tzname, tuple), len(time.tzname), all(isinstance(x, str) for x in time.tzname))
print(time.daylight in (0, 1), isinstance(time.timezone, int), isinstance(time.altzone, int))

# A string that does not match, or a field out of range, raises ValueError.
for bad, fmt in [("nope", "%Y-%m-%d"), ("2026-13-01", "%Y-%m-%d"), ("2026-02-30", "%Y-%m-%d")]:
    try:
        time.strptime(bad, fmt)
    except ValueError as e:
        print("VE:", e)
try:
    dt.datetime.strptime("x", "%Y")
except ValueError as e:
    print("dtVE:", e)
