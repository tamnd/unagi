# sys.path is the import search path. A compiled program resolves imports at
# build time so its contents are an implementation detail, but the attribute
# has to exist as a mutable list of strings: stdlib code iterates it, and
# linecache and warnings.warn_explicit reach through it. This checks the shape
# and the graceful degrade rather than the entries.
import sys

# It is a list of strings.
print(type(sys.path) is list)
print(all(isinstance(p, str) for p in sys.path))

# It is mutable: a prepend takes and reverts.
sys.path.insert(0, "/tmp/unagi_probe")
print(sys.path[0])
sys.path.pop(0)

# linecache walks sys.path and degrades to '' for a source file it cannot find.
import linecache
print(repr(linecache.getline("no_such_source_zzz.py", 1)))

# warnings.warn_explicit runs to completion; captured, it stays off stderr.
import warnings
with warnings.catch_warnings(record=True) as caught:
    warnings.simplefilter("always")
    warnings.warn_explicit("boom", UserWarning, "probe.py", 3)
    print(len(caught), caught[0].category.__name__, str(caught[0].message))
