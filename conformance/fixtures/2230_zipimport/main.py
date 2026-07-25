import zipimport
import _frozen_importlib_external as be
import _frozen_importlib as b
import importlib._bootstrap
import importlib._bootstrap_external

# zipimport imports the frozen bootstrap by name and subclasses _LoaderBasics at
# module scope, so the frozen aliases have to resolve to the real bootstrap.
print("imported")
print(zipimport.__name__)
print(zipimport.zipimporter.__name__)
print(issubclass(zipimport.zipimporter, be._LoaderBasics))

# The frozen names are the same module objects as the importlib bootstrap
# sources, matching CPython, so identity holds.
print(b is importlib._bootstrap)
print(be is importlib._bootstrap_external)

# The unpack helpers zipimport reads at import are real little-endian readers.
print(be._unpack_uint16(b"\x05\x00"))
print(be._unpack_uint32(b"\x01\x02\x00\x00"))
print(be._unpack_uint64(b"\x01\x00\x00\x00\x00\x00\x00\x00"))
