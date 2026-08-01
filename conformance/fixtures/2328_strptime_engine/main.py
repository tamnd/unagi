# _strptime is the shared engine behind time.strptime and
# datetime.datetime.strptime. This exercises the directive set, the derived
# fields (weekday and day of year), ISO week reconstruction, and the error
# paths, all of which are deterministic.
import time
import datetime
import _strptime

# Core directives through the low level _strptime, which returns the struct
# time tuple, the fractional seconds and the utc offset.
low = [
    ("2023-06-15", "%Y-%m-%d"),
    ("15/06/2023 14:30:45", "%d/%m/%Y %H:%M:%S"),
    ("Jun 15 2023", "%b %d %Y"),
    ("June 15, 2023", "%B %d, %Y"),
    ("2023-166", "%Y-%j"),
    ("02:30 PM", "%I:%M %p"),
    ("23", "%y"),
    ("2023-06-15T14:30:45", "%Y-%m-%dT%H:%M:%S"),
    ("100000", "%f"),
]
for s, fmt in low:
    tt, frac, gmtoff = _strptime._strptime(s, fmt)
    print("low", repr(s), tuple(tt), frac, gmtoff)

# time.strptime returns the struct_time; check weekday and yday derivation and
# the named day and month directives.
good = [
    ("2023-06-15 14:30:45", "%Y-%m-%d %H:%M:%S"),
    ("Tue Jun 15 14:30:45 2023", "%a %b %d %H:%M:%S %Y"),
    ("Thursday", "%A"),
    ("2023 24 1", "%G %V %u"),
    ("2023 W24 4", "%Y W%W %w"),
    ("2023%", "%Y%%"),
]
for s, fmt in good:
    print("time", repr(s), tuple(time.strptime(s, fmt)))

# Error paths: out of range day, bad month, literal mismatch, stray percent.
bad = [
    ("2023-02-29", "%Y-%m-%d"),
    ("2023-13-01", "%Y-%m-%d"),
    ("notadate", "%Y-%m-%d"),
    ("2023-06-15", "%Y/%m/%d"),
    ("2023-06-15", "%Y-%m-%d%"),
]
for s, fmt in bad:
    try:
        time.strptime(s, fmt)
        print("bad", repr(s), "NO ERROR")
    except ValueError as e:
        print("bad", repr(s), "ValueError", e)

# datetime.strptime builds a datetime from the same engine.
for s, fmt in [("2023-06-15 14:30", "%Y-%m-%d %H:%M"), ("Jun 2023", "%b %Y")]:
    print("dt", repr(s), datetime.datetime.strptime(s, fmt).isoformat())
