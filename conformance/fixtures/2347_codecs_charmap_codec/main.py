import codecs
import io


class Queue(object):
    def __init__(self, buffer):
        self._buffer = buffer

    def write(self, chars):
        self._buffer += chars

    def read(self, size=-1):
        if size < 0:
            s = self._buffer
            self._buffer = self._buffer[:0]
            return s
        s = self._buffer[:size]
        self._buffer = self._buffer[size:]
        return s


enc = "charmap"
s = "abc123"

# the generic charmap codec resolves through codecs.lookup like any other
print("name", codecs.lookup(enc).name)

# with no mapping it falls back to latin-1 on both directions
b, size = codecs.getencoder(enc)(s)
print("enc", b, size)
chars, size = codecs.getdecoder(enc)(b)
print("dec", chars, size)

# stream writer and reader carry the state one byte at a time
q = Queue(b"")
writer = codecs.getwriter(enc)(q)
encoded = b""
for c in s:
    writer.write(c)
    encoded += q.read()
print("stream enc", encoded)
q = Queue(b"")
reader = codecs.getreader(enc)(q)
decoded = ""
for c in encoded:
    q.write(bytes([c]))
    decoded += reader.read()
print("stream dec", decoded)

# incremental encoder and decoder
encoder = codecs.getincrementalencoder(enc)()
er = b""
for c in s:
    er += encoder.encode(c)
er += encoder.encode("", True)
decoder = codecs.getincrementaldecoder(enc)()
dr = ""
for c in er:
    dr += decoder.decode(bytes([c]))
dr += decoder.decode(b"", True)
print("incr", er, dr)

# iterencode and iterdecode round trip, including the empty string
print("iter", "".join(codecs.iterdecode(codecs.iterencode(s, enc), enc)))
print("iter empty", repr("".join(codecs.iterdecode(codecs.iterencode("", enc), enc))))

# incremental with an errors argument threads through
encoder = codecs.getincrementalencoder(enc)("ignore")
er = b"".join(encoder.encode(c) for c in s)
decoder = codecs.getincrementaldecoder(enc)("ignore")
dr = "".join(decoder.decode(bytes([c])) for c in er)
print("incr ignore", er, dr)

# seek resets the reader state
big = "%s\n%s\n" % (10 * "abc123", 10 * "def456")
reader = codecs.getreader(enc)(io.BytesIO(big.encode(enc)))
for t in range(3):
    reader.seek(0, 0)
    print("seek", t, reader.read() == big)

# every byte decodes and round trips the way latin-1 does
raw = bytes(range(256))
text = raw.decode(enc)
print("dec256 len", len(text))
print("roundtrip", text.encode(enc) == raw)

# a code point past the latin-1 range reports the latin-1 fallback and span
try:
    "Ā".encode(enc)
except UnicodeEncodeError as e:
    print("encerr", e.encoding, e.start, e.end, e.reason)

# the decoder getstate and setstate protocol answers on the charmap codec
d = codecs.getincrementaldecoder(enc)()
first = d.decode(s.encode(enc)[:3])
st = d.getstate()
rest = d.decode(s.encode(enc)[3:], True)
print("state dec", first, rest, st)

# a decoder fetched with no argument and a non-bytes argument both raise
try:
    codecs.getdecoder(enc)()
except TypeError:
    print("bad dec noarg TypeError")
try:
    codecs.getdecoder(enc)(42)
except TypeError:
    print("bad dec 42 TypeError")
try:
    codecs.getencoder(enc)()
except TypeError:
    print("bad enc noarg TypeError")
