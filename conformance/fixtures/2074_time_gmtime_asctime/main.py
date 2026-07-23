# The epoch conversions the pure-Python datetime reaches for its timestamp
# constructors: time.gmtime breaks a timestamp into a UTC struct_time, asctime
# renders one, and mktime folds a local struct_time back to a timestamp.
# gmtime is UTC and so identical on every host; localtime and mktime read the
# host zone, so only their round trip, which cancels the zone, is checked here.

import time

# gmtime is the UTC breakdown, with tm_zone fixed to UTC and tm_gmtoff to zero.
g0 = time.gmtime(0)
print(g0)
print(g0.tm_zone, g0.tm_gmtoff, g0.tm_wday, g0.tm_yday)
g = time.gmtime(1700000000)
print(g)
print(g.tm_year, g.tm_mon, g.tm_mday, g.tm_hour, g.tm_min, g.tm_sec)
print(time.gmtime(1234567890))

# asctime renders a struct_time or nine-tuple to the fixed 24-column form.
print(time.asctime(time.gmtime(1700000000)))
print(time.asctime((2026, 3, 4, 9, 5, 7, 2, 63, 0)))
print(time.asctime(time.gmtime(0)))

# strftime over a gmtime result threads the two together.
print(time.strftime("%Y-%m-%d %H:%M:%S UTC", time.gmtime(1700000000)))

# localtime and mktime read the host zone, but mktime(localtime(t)) cancels it
# and returns the original timestamp on any host.
for t in [0.0, 1700000000.0, 1234567890.0, 100.0]:
    print("roundtrip", t, time.mktime(time.localtime(t)) == t)

# datetime's clock constructor rides on localtime; it returns a datetime whose
# value is the current time, so only its type is host independent.
import datetime as dt

print(type(dt.datetime.now()).__name__)

# gmtime rejects a non-number the way CPython does.
try:
    time.gmtime("x")
except TypeError as e:
    print("gmtime:", e)
