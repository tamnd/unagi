# The struct p format is a Pascal string: a leading length byte followed by that
# many content bytes, the whole field a fixed width. A zero-width 0p field is a
# degenerate case with no length byte at all, so packing gives empty bytes and
# unpacking an empty buffer must return empty bytes rather than reading a length
# byte that is not there (that read used to run off the end of the buffer and
# panic). This pins the p surface across widths and both byte orders, alongside
# the neighbouring s (raw bytes) and c (single char) formats.
import struct

# The p format across widths: 0p has no length byte, a width-N field stores at
# most N-1 content bytes and the length byte is clamped to that.
for width in range(0, 12):
    fmt = ">" + str(width) + "p"
    packed = struct.pack(fmt, b"helloworld")
    print(fmt, "size", struct.calcsize(fmt), "packed", packed, "unpacked", struct.unpack(fmt, packed))

# Both byte orders agree on the p layout since the length byte is a single byte.
for order in (">", "<", "!", "="):
    fmt = order + "5p"
    packed = struct.pack(fmt, b"abcd")
    print(fmt, packed, struct.unpack(fmt, packed))

# A 0p field mixed with other fields packs and unpacks with no length byte for
# the empty one.
mixed = struct.pack(">0p4sb", b"ignored", b"data", 7)
print("mixed packed:", mixed)
print("mixed unpacked:", struct.unpack(">0p4sb", mixed))

# The s format zero-pads or truncates to its width, and 0s is empty.
for fmt in (">0s", ">1s", ">4s", ">8s"):
    packed = struct.pack(fmt, b"abcd")
    print(fmt, packed, struct.unpack(fmt, packed))

# A p field shorter than the content stores the clamped length in its first byte.
packed = struct.pack(">3p", b"abcdef")
print(">3p packed:", packed, "len byte:", packed[0], "unpacked:", struct.unpack(">3p", packed))

# calcsize agrees for a repeated p and a run of zero-width fields.
print("calcsize 0p0p0p:", struct.calcsize(">0p0p0p"))
print("unpack 0p0p0p:", struct.unpack(">0p0p0p", b""))
