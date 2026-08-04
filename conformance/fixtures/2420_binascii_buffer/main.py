# The binascii codecs accept any object that exposes a read-only buffer, not just
# bytes: a memoryview, an array.array and a bytearray all read the same as the
# equivalent bytes. unagi only accepted bytes, bytearray and str, so a memoryview
# or an array raised a bytes-like TypeError where CPython reads the bytes behind
# the buffer. This pins buffer acceptance across the b2a encoders and the a2b
# decoders, that a str still decodes on the a2b side, and that a non-buffer object
# (int, list) is still rejected.
import binascii
import array

mv = memoryview(b"abcd")
a = array.array("b", [97, 98, 99, 100])

# The b2a encoders read a buffer and produce the same output as over bytes.
print("hexlify mv:", binascii.hexlify(mv))
print("hexlify array:", binascii.hexlify(a))
print("b2a_base64 mv:", binascii.b2a_base64(mv))
print("b2a_qp array:", binascii.b2a_qp(a))
print("crc32 mv:", binascii.crc32(mv))
print("crc_hqx array:", binascii.crc_hqx(a, 0))

# The a2b decoders read a buffer too, and still take an ASCII str.
print("a2b_hex mv:", binascii.a2b_hex(memoryview(b"61626364")))
print("a2b_hex array:", binascii.a2b_hex(array.array("b", [54, 49, 54, 50])))
print("a2b_base64 mv:", binascii.a2b_base64(memoryview(b"YWJjZA==")))
print("unhexlify mv:", binascii.unhexlify(memoryview(b"61626364")))
print("a2b_hex str:", binascii.a2b_hex("6162"))

# A bytearray reads as its bytes.
print("hexlify bytearray:", binascii.hexlify(bytearray(b"xy")))

# A malformed buffer still raises the codec's own error.
try:
    binascii.a2b_hex(memoryview(b"6162x"))
except binascii.Error as e:
    print("odd/bad:", e)

# A non-buffer object is still a bytes-like TypeError.
for bad in (42, [1, 2, 3]):
    try:
        binascii.hexlify(bad)
    except TypeError as e:
        print("reject", type(bad).__name__ + ":", e)
