import codecs

# ord() now accepts a length-one bytes, which is what quopri.py relies on when it
# quotes a single byte. quopri_codec is a bytes-to-bytes transform reached through
# codecs.encode/decode.
print("ord_bytes", ord(b"A"), ord(bytearray(b"z")))

data = b"a=b c\td\n" + bytes(range(0x80, 0x90)) + b" trailing "
enc = codecs.encode(data, "quopri_codec")
print("enc", enc)
print("rt", codecs.decode(enc, "quopri_codec") == data)
print("dec", codecs.decode(b"a=3Db=20c", "quopri_codec"))
print("plain", codecs.encode(b"plain ascii text", "quopri_codec"))
print("softbreak", codecs.decode(b"long=\nline", "quopri_codec"))
