# escape_encode and readbuffer_encode are two _codecs C accelerators the codecs
# module re-exports through `from _codecs import *`. escape_encode renders a bytes
# object with the C bytes-repr escapes (single quote and backslash escaped, the
# \t \n \r names, \xNN for every byte outside printable ASCII) and returns the
# (escaped bytes, input length) pair. readbuffer_encode hands back the raw bytes
# behind a bytes-like object, or a str encoded as UTF-8, paired with the byte
# length. This pins both against CPython including the argument-type errors.
import codecs
import array

# escape_encode over a spread of inputs, including the full 256-byte range.
enc_cases = [
    b"",
    b"foobar",
    b"spam\0eggs",
    b"a'b",
    b"b\\c",
    b"c\nd",
    b"d\re",
    b"f\x7fg",
    bytes(range(256)),
]
for data in enc_cases:
    print("enc", codecs.escape_encode(data))

# readbuffer_encode over an empty str, a multibyte str, an array, raw bytes and a
# bytearray. A str is encoded UTF-8 and the length is the byte count.
print("rb-empty", codecs.readbuffer_encode(""))
print("rb-str", codecs.readbuffer_encode("café 中"))
print("rb-array", codecs.readbuffer_encode(array.array("b", b"spam")))
print("rb-bytes", codecs.readbuffer_encode(b"\x00\x01\xff"))
print("rb-bytearray", codecs.readbuffer_encode(bytearray(b"abc")))

# Argument-type and missing-argument errors match the C wording.
type_cases = [
    ("ee-str", codecs.escape_encode, "x"),
    ("ee-ba", codecs.escape_encode, bytearray(b"x")),
    ("rb-int", codecs.readbuffer_encode, 42),
]
for label, fn, arg in type_cases:
    try:
        fn(arg)
        print(label, "no-raise")
    except TypeError as e:
        print(label, "TypeError", e)

for label, fn in [("ee-none", codecs.escape_encode), ("rb-none", codecs.readbuffer_encode)]:
    try:
        fn()
        print(label, "no-raise")
    except TypeError as e:
        print(label, "TypeError", e)
