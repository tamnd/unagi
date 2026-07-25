import pickle
cases = [0, 255, 32767, -256, -32768, -128, 127, 1, -1, 256, -1000000,
         123456789012345678901234567890, -98765432109876543210, 2**64, -(2**64)]
for x in cases:
    b = pickle.encode_long(x)
    assert pickle.decode_long(b) == x, x
    print(x, b.hex())
print(pickle.decode_long(b''))
print(pickle.encode_long(0) == b'')
print(pickle.decode_long(bytearray(b'\xff\x00')))
