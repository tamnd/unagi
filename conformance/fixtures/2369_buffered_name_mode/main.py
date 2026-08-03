import io
import os

# A buffered stream from open() exposes the name and mode of the raw file it
# wraps, the way CPython's BufferedReader/BufferedWriter delegate to the raw
# stream. The name is a full path that depends on where this runs, so the checks
# use its basename to stay path-independent.
path = "buffered_probe.bin"

wb = open(path, "wb")
print("wb type:", type(wb).__name__)
print("wb name basename:", os.path.basename(wb.name))
print("wb name is str:", isinstance(wb.name, str))
print("wb mode:", wb.mode)
wb.write(b"payload")
wb.close()

rb = open(path, "rb")
print("rb type:", type(rb).__name__)
print("rb name basename:", os.path.basename(rb.name))
print("rb mode:", rb.mode)
# The name reads the same before and after a read, since it rides on the raw file.
rb.read(3)
print("rb name after read:", os.path.basename(rb.name))
rb.close()

# A buffered stream over a BytesIO has no underlying file name, so the access
# propagates the raw's AttributeError.
buf = io.BufferedReader(io.BytesIO(b"data"))
try:
    buf.name
except AttributeError:
    print("bytesio name raises:", True)

# The same delegation drives the gettext pattern getattr(fp, 'name', ''): a real
# file hands back its name, a nameless stream falls back to the default.
print("getattr real:", os.path.basename(getattr(open(path, "rb"), "name", "")))
print("getattr nameless:", repr(getattr(io.BufferedReader(io.BytesIO(b"")), "name", "")))

os.remove(path)
