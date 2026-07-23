# Reading a compiled pattern's method off the instance binds it as a callable,
# so re.compile(...).sub hands back something you can call rather than raising.
# fnmatch opens with _re_setops_sub = re.compile(...).sub at import, so the whole
# module, and glob through it, only load once the bound read works.

import re
import fnmatch
import glob

p = re.compile(r'([&~|])')

# The bound method calls the same way the direct p.sub(...) does.
print(p.sub(r'[\1]', 'a&b|c~d'))
print(p.subn('_', 'a&b|c~d'))

# Bound once and stored, the way fnmatch keeps _re_setops_sub.
sub = p.sub
print(sub('#', 'x|y'))

# Every method a compiled pattern dispatches also binds as an attribute read.
for name in ['match', 'fullmatch', 'search', 'findall', 'finditer', 'sub', 'subn', 'split']:
    print(name, callable(getattr(p, name)))

# fnmatch and glob import only because the bound read works; exercise fnmatch.
print(fnmatch.fnmatch('Foo.TXT', '*.txt'))
print(fnmatch.fnmatchcase('Foo.TXT', '*.txt'))
print(fnmatch.filter(['a.py', 'b.txt', 'c.py', 'README'], '*.py'))
print(fnmatch.translate('*.txt'))
print(type(glob.glob).__name__)

# An attribute a pattern does not carry still raises.
try:
    p.nope
except AttributeError as e:
    print("AE:", e)
