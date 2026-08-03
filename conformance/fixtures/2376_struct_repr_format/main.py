import struct


# A Struct reprs as its type name applied to the repr of its format string, and
# the format attribute is a str across every accepted format.
for f in [">i", "", "<10s2H", "@ci", ">2i0b", "!4b"]:
    s = struct.Struct(f)
    print(repr(s), "| format=", repr(s.format), "| type=", type(s.format).__name__)

# A bytes format is normalised to a str, so both the repr and the format
# attribute report the decoded str, not the bytes.
sb = struct.Struct(b">d")
print(repr(sb), "| format=", repr(sb.format), "| type=", type(sb.format).__name__)

# A subclass reprs under its own name.
class MyStruct(struct.Struct):
    pass


print(repr(MyStruct(">i")))
print(repr(MyStruct(b"<f")))

# The repr survives round-tripping through str() and is usable in a format call.
s = struct.Struct(">hhl")
print(str(s))
print("packed size is {} for {!r}".format(s.size, s))

# A Struct still works normally after all the repr traffic.
s = struct.Struct(">2i")
print(s.pack(1, 2).hex(), s.unpack(b"\x00\x00\x00\x01\x00\x00\x00\x02"))
