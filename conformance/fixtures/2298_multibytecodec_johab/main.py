# johab on the _multibytecodec engine, the Korean combinational encoding, driven
# through the vendored encodings/johab.py. johab decodes ascii as single bytes
# and treats every high byte as a lead that selects a two-byte character, so it
# is a fixed-width two-byte codec the generic mbTableCodec drives. This exercises
# the stateless encode/decode, the incremental encoder and decoder (including a
# character split across a chunk boundary), the stream reader and writer, and the
# strict/ignore/replace error handling, all of which must match CPython byte for
# byte.
import codecs
import io

text = "한국어 abc 123 안녕 조합형 테스트."

data = text.encode("johab")
print("encoded hex:", data.hex())
print("decoded eq:", data.decode("johab") == text)
print("codecs.encode eq:", codecs.encode(text, "johab") == data)
print("codecs.decode eq:", codecs.decode(data, "johab") == text)

info = codecs.lookup("johab")
print("info name:", info.name)
print("info encode eq:", info.encode(text)[0] == data)
print("info decode eq:", info.decode(data)[0] == text)

enc = codecs.getincrementalencoder("johab")()
chunks = b""
for ch in text:
    chunks += enc.encode(ch)
chunks += enc.encode("", True)
print("incremental encode eq:", chunks == data)

dec = codecs.getincrementaldecoder("johab")()
out = ""
step = 3
for i in range(0, len(data), step):
    out += dec.decode(data[i:i + step], False)
out += dec.decode(b"", True)
print("incremental decode eq:", out == text)

# A lead byte with no trailing byte buffers until the next chunk completes it.
dec2 = codecs.getincrementaldecoder("johab")()
lead = "한".encode("johab")
print("buffered lead:", repr(dec2.decode(lead[:1], False)))
print("completed pair:", dec2.decode(lead[1:], True) == "한")

buf = io.BytesIO()
writer = codecs.getwriter("johab")(buf)
writer.write(text)
writer.flush()
print("stream bytes eq:", buf.getvalue() == data)
buf.seek(0)
reader = codecs.getreader("johab")(buf)
print("stream read eq:", reader.read() == text)

# Decode error handling. johab has no illegal standalone byte (every high byte
# is a lead), so a lead with a bad trail is illegal one byte wide and a lead at
# end of input is incomplete.
try:
    bytes([0xd8, 0x20]).decode("johab")
except UnicodeDecodeError as e:
    print("decode bad trail:", e)
try:
    bytes([0x41, 0x84]).decode("johab")
except UnicodeDecodeError as e:
    print("decode incomplete:", e)
bad = bytes([0xd8, 0x20]) + "한".encode("johab")
print("decode ignore eq:", bad.decode("johab", "ignore") == " 한")
print("decode replace eq:", bad.decode("johab", "replace") == "� 한")

# Encode error handling. An emoji outside the BMP has no johab mapping.
try:
    "a\U0001f600b".encode("johab")
except UnicodeEncodeError as e:
    print("encode strict:", e)
print("encode ignore eq:", "a\U0001f600b".encode("johab", "ignore") == b"ab")
print("encode replace eq:", "a\U0001f600b".encode("johab", "replace") == b"a?b")
