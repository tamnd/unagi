# bool.from_bytes narrows int.from_bytes to the class it is called on, so the
# result is a True/False singleton, not a bare 0/1 int.
print(bool.from_bytes(b'\x00' * 8, 'big'))
print(bool.from_bytes(b'\x00' * 8, 'big') is False)
print(bool.from_bytes(b'abcd', 'little'))
print(bool.from_bytes(b'abcd', 'little') is True)
print(type(bool.from_bytes(b'\x00', 'big')).__name__)

# Any nonzero magnitude is True, whatever the byte order or sign.
print(bool.from_bytes(b'\x02', 'big'))
print(bool.from_bytes(b'\xff', 'big', signed=True))
print(bool.from_bytes(b'', 'big'))

# int.from_bytes still spells the integer verbatim.
print(int.from_bytes(b'\x00', 'big'), int.from_bytes(b'\x02', 'big'))
