# int() accepts bytes and bytearray as well as str. Each byte is read as its
# latin-1 code point before the digits are parsed.

# With an explicit base.
print(int(b"1010", 2))
print(int(bytearray(b"ff"), 16))
print(int(b"  0x2a  ", 16))
print(int(b"-17", 10))
print(int(b"777", 8))
print(int(b"z", 36))
print(int(b"0b111", 0))

# With a single argument, parsed as base-10 text.
print(int(b"10"))
print(int(bytearray(b"  -42  ")))
print(int(b"1_000"))

# A bad digit still raises the same ValueError as the str path.
try:
    int(b"12", 2)
except ValueError as e:
    print("bad:", e)

# A memoryview is not accepted for the base form, only str, bytes, bytearray.
try:
    int(memoryview(b"777"), 8)
except TypeError as e:
    print("mv:", e)
