# _locale + locale.py in the C locale. Everything here is deterministic and
# platform independent: raw LC_* category integers and getpreferredencoding
# are left out because their values differ between macOS and glibc CPython.
import locale

locale.setlocale(locale.LC_ALL, "C")

# localeconv returns the C locale convention table.
conv = locale.localeconv()
for key in sorted(conv):
    print(key, repr(conv[key]))

# The category constants exist and setlocale(cat) reports the C locale.
for name in ("LC_CTYPE", "LC_COLLATE", "LC_TIME", "LC_MONETARY", "LC_NUMERIC"):
    cat = getattr(locale, name)
    print(name, "->", repr(locale.setlocale(cat)))

# String collation in C locale is plain byte ordering.
print("strcoll", locale.strcoll("abc", "abd"), locale.strcoll("b", "a"), locale.strcoll("a", "a"))
print("strxfrm", repr(locale.strxfrm("hello")))

# Numeric parsing and formatting.
print("atoi", locale.atoi("12345"), locale.atoi("-9"))
print("atof", locale.atof("3.14"), locale.atof("-0.5"))
print("str", repr(locale.str(1234.5)), repr(locale.str(-0.0)), repr(locale.str(100.0)))
print("delocalize", repr(locale.delocalize("1234.5")))

# format_string with and without grouping (grouping is a no-op in C).
print(locale.format_string("%d", 1000000, grouping=True))
print(locale.format_string("%.2f", -1234.5))
print(locale.format_string("%s=%i", ("n", 42)))

# currency requires a locale with a currency symbol; C locale has none.
try:
    locale.currency(10)
except ValueError as e:
    print("currency ValueError", e)

# normalize resolves aliases to canonical name.dot.encoding form.
for name in ("en_US", "en", "de_DE.UTF-8", "zh_CN.gb2312", "uk_UA.KOI8-U", "C"):
    print("normalize", name, "->", locale.normalize(name))

# getlocale for a category returns a (language, encoding) pair.
print("getlocale", locale.getlocale(locale.LC_CTYPE))

# setlocale rejects an unknown locale name.
try:
    locale.setlocale(locale.LC_ALL, "no_such_locale.bogus")
except locale.Error as e:
    print("setlocale Error", e)
