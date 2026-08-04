# float.__round__ shares its decimal-exact rounding with the round() builtin, so
# (2.675).__round__(2) is 2.67 the way CPython's dtoa rounding gives, not the
# 2.68 a naive scale-and-round would land on. No digit count rounds to the
# nearest int; a count keeps the value a float. Halves round to even.

def show(f):
    try:
        print("ok", repr(f()))
    except Exception as e:
        print(type(e).__name__, e)

print(hasattr(1.5, "__round__"), hasattr(2.5, "__round__"))

# No argument or None rounds to the nearest int, ties to even.
print((2.5).__round__(), (3.5).__round__(), (-2.5).__round__(), (0.5).__round__())
print((2.5).__round__(None), (2.4).__round__(), (2.6).__round__())

# A digit count keeps the value a float, rounded through exact decimal math.
print((2.675).__round__(2), (2.5).__round__(0), (0.125).__round__(2))
print((-0.5).__round__(0), (1234.5678).__round__(-2), (1234.5678).__round__(2))
print((2.5e300).__round__(-305), (1.0).__round__(1000))

# The round() builtin agrees with the dunder on the same inputs.
print(round(2.675, 2), round(2.5), round(1234.5678, -2), round(-0.5), round(0.125, 2))

# An infinite or nan value rounds through with a digit count but cannot become
# an int with none.
print((float("inf")).__round__(2), (float("nan")).__round__(2))
show(lambda: (float("inf")).__round__())
show(lambda: (float("nan")).__round__())

# The error paths match CPython's wording.
show(lambda: (2.5).__round__(1.5))
show(lambda: (2.5).__round__(1, 2))
show(lambda: (1e308).__round__(-308))
