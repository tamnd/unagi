import codecs

# The bytes-to-bytes transform codecs go through codecs.encode/decode with a
# bytes argument rather than str.encode, so they exercise the registry dispatch
# for a non-str object.
data = b"Hello, World! " * 3

# base64_codec and hex_codec produce deterministic output, so the bytes are
# printed directly and checked against the oracle.
print("b64_enc", codecs.encode(data, "base64_codec"))
print("b64_rt", codecs.decode(codecs.encode(data, "base64_codec"), "base64_codec") == data)
print("b64_small", codecs.encode(b"abc", "base64_codec"))
print("b64_dec", codecs.decode(b"YWJj\n", "base64_codec"))

print("hex_enc", codecs.encode(data, "hex_codec"))
print("hex_bytes", codecs.encode(b"\x00\xff\x10\x7f", "hex_codec"))
print("hex_dec", codecs.decode(b"48656c6c6f", "hex_codec"))
print("hex_rt", codecs.decode(codecs.encode(data, "hex_codec"), "hex_codec") == data)

# zlib_codec compressed bytes are implementation dependent, so only the
# round-trip and the decode of a fixed CPython-produced blob are checked.
blob = b'x\x9c+\xcdKL\xcfT\xa8\xca\xc9LRH\xceOIMV(N\xcc-\xc8IU(H\xac\xcc\xc9OL\x01\x00\xb9\xcb\x0b\xb0'
print("zlib_dec", codecs.decode(blob, "zlib_codec"))
print("zlib_rt", codecs.decode(codecs.encode(data, "zlib_codec"), "zlib_codec") == data)
print("zlib_empty", codecs.decode(codecs.encode(b"", "zlib_codec"), "zlib_codec"))
