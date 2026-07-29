# time.strftime(format) with no time tuple renders the current localtime, the
# form test.support takes at import to probe for strftime width extensions
# (time.strftime("%4Y")). unagi required an explicit tuple and raised without
# one, so this stopped test.support from importing. The one-argument call now
# breaks down the current clock, matching time.strftime(format, time.localtime()).
import time

print("is str", isinstance(time.strftime("%Y"), str))

# The no-tuple render equals the explicit-localtime render for stable fields.
print("year", time.strftime("%Y") == time.strftime("%Y", time.localtime()))
print("date", time.strftime("%Y-%m-%d") == time.strftime("%Y-%m-%d", time.localtime()))
print("month name", time.strftime("%B") == time.strftime("%B", time.localtime()))

# The probe test.support runs must return a string rather than raise, whatever
# the platform makes of the width modifier.
print("ext probe", isinstance(time.strftime("%4Y"), str))

# An explicit tuple still works unchanged, and a fixed one pins exact output.
t = time.struct_time((2026, 7, 22, 10, 5, 30, 1, 203, 0))
print("fixed", time.strftime("%Y-%m-%d %H:%M:%S %A", t))
