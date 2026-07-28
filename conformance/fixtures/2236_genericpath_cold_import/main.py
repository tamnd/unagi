# genericpath as the cold entry import: it imports os, os imports posixpath, and
# posixpath runs `from genericpath import *` while genericpath is still half
# built. Default CPython survives this because its startup imports os first,
# which loads genericpath fully through the same cycle. The runtime does the
# equivalent, so importing genericpath first works.
import genericpath
print(genericpath.__name__)
print(genericpath.commonprefix(['flower', 'flow', 'flight']))
print(repr(genericpath.commonprefix([])))
print(hasattr(genericpath, 'exists'))

# os and its path module loaded through the same cycle and are fully usable.
import os
print(os.path.commonprefix(['/a/b', '/a/c']))
