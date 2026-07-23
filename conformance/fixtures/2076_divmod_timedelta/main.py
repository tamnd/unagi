# The divmod builtin dispatches to __divmod__/__rdivmod__ for a pair its number
# path cannot handle, so a timedelta, which defines __divmod__, divides by
# another timedelta. This is what the pure-Python datetime reaches for when it
# formats a fixed UTC offset, so tz-aware datetime string forms rely on it.

import datetime as dt

# divmod of two timedeltas returns the integer quotient and a timedelta
# remainder, the same result as the // and % operators.
a = dt.timedelta(hours=5, minutes=30)
b = dt.timedelta(minutes=15)
q, r = divmod(a, b)
print(q, str(r))
print(a // b, str(a % b))

c = dt.timedelta(days=2, hours=3, minutes=20)
d = dt.timedelta(hours=1)
print(divmod(c, d)[0], str(divmod(c, d)[1]))

# The tz-aware datetime string form folds the offset with divmod of two
# timedeltas, so these render only because the fallback above runs.
print(str(dt.datetime.fromtimestamp(0, dt.timezone.utc)))
print(str(dt.datetime.fromtimestamp(0, dt.timezone(dt.timedelta(hours=5, minutes=30)))))
print(str(dt.datetime.fromtimestamp(0, dt.timezone(dt.timedelta(hours=-8)))))
print(dt.datetime(2026, 7, 24, 13, 30, tzinfo=dt.timezone(dt.timedelta(hours=5, minutes=30, seconds=45))).isoformat())

# A pair with no __divmod__ slot still raises the unsupported-operand TypeError.
try:
    divmod([], [])
except TypeError as e:
    print("TE:", e)
try:
    divmod("a", "b")
except TypeError as e:
    print("TE:", e)
