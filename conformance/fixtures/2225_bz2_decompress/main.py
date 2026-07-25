import bz2

# A fixed bzip2 stream produced by CPython's bz2.compress. Only the decompression
# half is exercised here: Go's standard library carries a bzip2 decompressor but
# no compressor, so this native _bz2 lands reading for real and its output is
# byte-identical to CPython. The write half raises NotImplementedError, which is
# not printed since CPython would instead succeed.
data = (
    b'BZh91AY&SY2\x0fC\xf7\x00\x01g\xd9\x80\x00\x10@\x01\x10\x00>\xa7\xdf\x10'
    b' \x00\x90(\xd1\x904i\x91\xa0R\xaai\xa3F\x9a\x0352\'bfMI\xe4\x9c\xc9\x82'
    b'd&\x84\xcc\x99\x93"`\x9e\t\xb1?\x13bz\'\xb2nN\t\xc8\x9b\x89\x82x&\xa4\xf2'
    b'N\t\x82hN\x84\xf6M\x89\xc14&\t\xfc]\xc9\x14\xe1B@\xc8=\x0f\xdc'
)

# One-shot decompress round trips the whole stream.
out = bz2.decompress(data)
print(len(out))
print(out[:38].decode())
print(out == b'unagi native bz2 decompression slice.\n' * 20)

# Incremental decompress reports the stream end and an empty unused tail.
dec = bz2.BZ2Decompressor()
whole = dec.decompress(data)
print(whole == out)
print(dec.eof)
print(dec.unused_data == b'')
print(dec.needs_input)

# A length-limited call hands back only the requested prefix and keeps eof false
# until the rest is drained.
d2 = bz2.BZ2Decompressor()
head = d2.decompress(data, 5)
print(head.decode())
print(d2.eof)
tail = d2.decompress(b'')
print(len(tail) == len(out) - 5)
print(d2.eof)

# The compressor constructs the way CPython allows.
print(bz2.BZ2Compressor(9) is not None)
