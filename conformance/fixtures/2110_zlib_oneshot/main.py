# The native zlib accelerator lands the one-shot codec and the checksums. The
# compressed bytes are implementation-defined, so the test relies on the round
# trip and the exact, algorithm-defined checksums, plus decompressing a blob
# that real C zlib produced, embedded as a hex literal.
import zlib

data = b"hello hello hello world " * 8

# Round trips through the zlib, raw-deflate, and gzip framings the wbits select,
# including the high range that auto-detects zlib versus gzip.
print(zlib.decompress(zlib.compress(data)) == data)
print(zlib.decompress(zlib.compress(data, 9)) == data)
print(zlib.decompress(zlib.compress(data, 1)) == data)
print(zlib.decompress(zlib.compress(data, 0)) == data)
print(zlib.decompress(zlib.compress(data, wbits=-15), -15) == data)
print(zlib.decompress(zlib.compress(data, wbits=31), 31) == data)
print(zlib.decompress(zlib.compress(data, wbits=31), 47) == data)

# CRC-32 and Adler-32 are exact and chain: a checksum fed a prior value equals
# the checksum of the concatenation.
print(zlib.crc32(b"hello"))
print(zlib.crc32(b"world", zlib.crc32(b"hello")) == zlib.crc32(b"helloworld"))
print(zlib.adler32(b"hello"))
print(zlib.adler32(b"world", zlib.adler32(b"hello")) == zlib.adler32(b"helloworld"))
print(zlib.crc32(b""), zlib.adler32(b""))

# Decompress a stream real C zlib emitted (level 9, zlib framing).
blob = bytes.fromhex(
    "78da0bc94855282ccd4cce56482aca2fcf5348cbaf50c82acd2d2856c82f4b2d"
    "5228014ae72456552aa4e4a703005bdc0fda"
)
print(zlib.decompress(blob))

# The exception and the constants.
print(zlib.error.__name__)
print(zlib.MAX_WBITS, zlib.DEFLATED, zlib.DEF_MEM_LEVEL, zlib.DEF_BUF_SIZE)
print(zlib.Z_BEST_COMPRESSION, zlib.Z_BEST_SPEED, zlib.Z_DEFAULT_COMPRESSION, zlib.Z_NO_COMPRESSION)
print(zlib.Z_FINISH, zlib.Z_SYNC_FLUSH, zlib.Z_DEFAULT_STRATEGY)

# Bad input raises zlib.error.
try:
    zlib.decompress(b"this is not a zlib stream")
    print("no error")
except zlib.error:
    print("caught zlib.error")
