# The integer conversion dunders operator.index and the pure-Python datetime and
# calendar reach on an int: __index__, __int__, __trunc__ and __float__. A bool is
# an int, so its dunders answer a plain int. Every value here is a fixed integer or
# a fixed date, identical on every host.

# The four conversion dunders read off an int, and off a bool as its int value.
print((5).__index__(), True.__index__(), (0).__index__())
print((5).__int__(), False.__int__())
print((-3).__trunc__(), (7).__trunc__())
print((5).__float__(), (-2).__float__())

# operator.index is the public door onto __index__ that datetime and calendar use.
import operator
print(operator.index(7), operator.index(True))

# datetime builds a date through int.__index__ on each field and answers it back.
import datetime as dt
d = dt.date(2026, 7, 24)
print(d.isoformat(), d.weekday(), d.toordinal())
print((dt.date(2026, 12, 31) - d).days)
print(dt.date(2026, 7, 24) + dt.timedelta(days=10))

# calendar leans on the same index path for its month and weekday math.
import calendar
print(calendar.isleap(2024), calendar.isleap(2026))
print(calendar.monthrange(2026, 7), calendar.weekday(2026, 7, 24))

# The bad-argument paths raise the CPython error text.
try:
    (5).__index__(1)
except TypeError as e:
    print("index:", e)
try:
    (5).__float__(1)
except TypeError as e:
    print("float:", e)
try:
    (5).__trunc__(1)
except TypeError as e:
    print("trunc:", e)
