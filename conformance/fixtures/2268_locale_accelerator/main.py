"""The _locale C extension backs the public locale module: locale.py does
`from _locale import *` and only falls back to a minimal "C"-only emulation when
that import fails. This exercises _locale directly, restricting itself to the
portable "C" locale so every value is fixed by the C standard and identical on
every platform: setting the C locale, the C-locale localeconv parameters, the
CHAR_MAX sentinel, that an unknown locale raises Error, and that Error is an
Exception subclass. The OS-backed strcoll/strxfrm and environment-driven
setlocale results are platform-specific and deliberately not exercised."""

import _locale

# Force the portable C locale so the parameters below are deterministic
# everywhere; setting "C" always echoes back "C".
print("setlocale C:", _locale.setlocale(_locale.LC_ALL, "C"))

# localeconv() for the C locale, printed in sorted key order for a stable dump.
conv = _locale.localeconv()
for key in sorted(conv):
    print(f"{key} = {conv[key]!r}")

print("CHAR_MAX:", _locale.CHAR_MAX)

# An unknown locale raises _locale.Error.
try:
    _locale.setlocale(_locale.LC_ALL, "zz_ZZ.INVALID")
except _locale.Error:
    print("Error raised for unknown locale")

print("Error subclass Exception:", issubclass(_locale.Error, Exception))
