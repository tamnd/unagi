# The streaming half of the zlib accelerator: compressobj and decompressobj, plus
# the gzip module that rides on them. Compressed bytes are implementation-defined
# so the test leans on round trips, the stream-end flag, and the trailing bytes
# that survive a stream, never on the raw compressed output.
import zlib

data = b"streaming zlib payload " * 40

# compressobj feeds data in chunks and flush() ends the stream; decompressobj
# reads it all back.
co = zlib.compressobj()
comp = co.compress(data[:100]) + co.compress(data[100:]) + co.flush()
do = zlib.decompressobj()
print(do.decompress(comp) + do.flush() == data)
print(do.eof)

# Raw deflate through the objects, selected by a negative wbits.
rc = zlib.compressobj(6, zlib.DEFLATED, -15)
raw = rc.compress(data) + rc.flush()
rd = zlib.decompressobj(-15)
print(rd.decompress(raw) == data, rd.eof)

# Z_SYNC_FLUSH pushes pending output but leaves the stream open for more.
c2 = zlib.compressobj()
b1 = c2.compress(b"first") + c2.flush(zlib.Z_SYNC_FLUSH)
b2 = c2.compress(b"second") + c2.flush()
print(zlib.decompress(b1 + b2) == b"firstsecond")

# unused_data holds whatever trailed the stream.
d3 = zlib.decompressobj()
print(d3.decompress(comp + b"TAIL") == data)
print(d3.unused_data)

# A length-limited decompress caps output; the rest drains on the next call.
d4 = zlib.decompressobj()
first = d4.decompress(comp, 50)
rest = d4.decompress(d4.unconsumed_tail)
print(len(first), (first + rest) == data)

# gzip.compress and gzip.decompress ride on the streaming codec.
import gzip
print(gzip.decompress(gzip.compress(data)) == data)

# A finished compressor rejects more input with zlib.error.
cf = zlib.compressobj()
cf.compress(b"x")
cf.flush()
try:
    cf.compress(b"y")
    print("no error")
except zlib.error:
    print("caught zlib.error")
