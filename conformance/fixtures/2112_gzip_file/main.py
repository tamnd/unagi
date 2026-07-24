# The private zlib._ZlibDecompressor the file objects read through, and GzipFile
# reading and writing a real file on top of it. Compressed bytes are
# implementation-defined so the test leans on round trips and the stream flags,
# never on the raw compressed output.
import zlib

data = b"hello gzip world " * 40
raw = zlib.compress(data, 6, -15)

# Feeding input in two chunks exercises needs_input: after a partial stream the
# decompressor asks for more, and eof stays false until the whole member drains.
d = zlib._ZlibDecompressor(-zlib.MAX_WBITS)
out = d.decompress(raw[:5])
print(d.needs_input, d.eof)
out += d.decompress(raw[5:])
print(out == data, d.eof, d.needs_input)

# A length-limited call caps output and keeps eof false while bytes are still
# owed, so the reader never sees the stream end before it has read the tail.
d2 = zlib._ZlibDecompressor(-zlib.MAX_WBITS)
part = d2.decompress(raw, 5)
print(len(part), d2.eof)
rest = d2.decompress(b"", 1000)
print((part + rest) == data, d2.eof)

# Bytes past the stream end surface through unused_data once the stream ends.
d3 = zlib._ZlibDecompressor(-zlib.MAX_WBITS)
d3.decompress(raw + b"TAIL")
print(d3.unused_data)

# GzipFile riding on the decompressor: write a file, read it back, and read it a
# line at a time to walk the member through the file object.
import gzip
import os
import tempfile

lines = b"line one\nline two\nline three\n"
path = os.path.join(tempfile.mkdtemp(), "sample.gz")
with gzip.open(path, "wb") as f:
    f.write(data)
with gzip.open(path, "rb") as f:
    print(f.read() == data)
with gzip.open(path, "wb") as f:
    f.write(lines)
with gzip.open(path, "rb") as f:
    print(f.readline() == b"line one\n")
    print(f.read() == b"line two\nline three\n")
