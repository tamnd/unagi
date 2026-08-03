import locale

# Under the portable C locale, strcoll is a plain ordering and strxfrm leaves the
# string unchanged.
print("strcoll a<b:", locale.strcoll("a", "b") < 0)
print("strcoll a==a:", locale.strcoll("a", "a") == 0)
print("strcoll b>a:", locale.strcoll("b", "a") > 0)


def raises(exc, fn, *args):
    try:
        fn(*args)
    except exc:
        return True
    except BaseException:
        return False
    return False


# An embedded null in either argument raises ValueError, since the C functions
# take a NUL-terminated string.
print("strcoll null arg1:", raises(ValueError, locale.strcoll, "a\0", "a"))
print("strcoll null arg2:", raises(ValueError, locale.strcoll, "a", "a\0"))
print("strxfrm null:", raises(ValueError, locale.strxfrm, "a\0"))

# Every valid category can be queried without raising and reports a locale name.
# The exact string is the process's ambient locale, so this checks the shape, not
# the value.
cats = [
    locale.LC_ALL,
    locale.LC_COLLATE,
    locale.LC_CTYPE,
    locale.LC_MONETARY,
    locale.LC_NUMERIC,
    locale.LC_TIME,
]
print("all query str:", all(isinstance(locale.setlocale(c), str) for c in cats))

# A category integer outside the known LC_* range raises locale.Error, the guard
# from bug #7419.
print("bad category:", raises(locale.Error, locale.setlocale, 12345))

# An unsupported locale name raises locale.Error too.
print("unsupported locale:", raises(locale.Error, locale.setlocale, locale.LC_ALL, "zz_ZZ.INVALID"))
