# sys carries the pinned CPython's identity values, so a floor module that gates
# on the version or reads a limit sees what it would under CPython 3.14.6. The
# stable, cross-platform ones are checked here; version-string, platform, argv,
# and path are host or build dependent and stay out of the golden.
import sys

# A version gate is the common shape: comparison against a tuple and indexing.
print("ge 3.8:", sys.version_info >= (3, 8))
print("lt 4:", sys.version_info < (4, 0))
print("triple:", sys.version_info[0], sys.version_info[1], sys.version_info[2])
print("level:", sys.version_info[3], sys.version_info[4])
print("len:", len(sys.version_info))

print("hexversion:", sys.hexversion == 0x030E06F0)
print("maxsize:", sys.maxsize)
print("maxunicode:", sys.maxunicode)
print("byteorder:", sys.byteorder)
