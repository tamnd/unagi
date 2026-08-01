# cp949 on the _multibytecodec engine, the Unified Hangul Code superset of
# euc_kr, driven through the vendored encodings/cp949.py. cp949 decodes ascii as
# single bytes and treats every high byte as a lead that selects a two-byte
# character, adding the modern Hangul syllables euc_kr leaves out, so it is a
# fixed-width two-byte codec the generic mbTableCodec drives. This exercises the
# stateless encode/decode, the incremental encoder and decoder (including a
# character split across a chunk boundary), the stream reader and writer, and the
# strict/ignore/replace error handling, all of which must match CPython byte for
# byte.
import codecs
import io

text = "한국어 abc 123 뷁 꿹 특수문자 테스트."

data = text.encode("cp949")
print("encoded hex:", data.hex())
print("decoded eq:", data.decode("cp949") == text)
print("codecs.encode eq:", codecs.encode(text, "cp949") == data)
print("codecs.decode eq:", codecs.decode(data, "cp949") == text)

info = codecs.lookup("cp949")
print("info name:", info.name)
print("info encode eq:", info.encode(text)[0] == data)
print("info decode eq:", info.decode(data)[0] == text)

enc = codecs.getincrementalencoder("cp949")()
chunks = b""
for ch in text:
    chunks += enc.encode(ch)
chunks += enc.encode("", True)
print("incremental encode eq:", chunks == data)

dec = codecs.getincrementaldecoder("cp949")()
out = ""
step = 3
for i in range(0, len(data), step):
    out += dec.decode(data[i:i + step], False)
out += dec.decode(b"", True)
print("incremental decode eq:", out == text)

# A lead byte with no trailing byte buffers until the next chunk completes it.
dec2 = codecs.getincrementaldecoder("cp949")()
lead = "뷁".encode("cp949")
print("buffered lead:", repr(dec2.decode(lead[:1], False)))
print("completed pair:", dec2.decode(lead[1:], True) == "뷁")

buf = io.BytesIO()
writer = codecs.getwriter("cp949")(buf)
writer.write(text)
writer.flush()
print("stream bytes eq:", buf.getvalue() == data)
buf.seek(0)
reader = codecs.getreader("cp949")(buf)
print("stream read eq:", reader.read() == text)

# Decode error handling. cp949 has no illegal standalone byte (every high byte
# is a lead), so a lead with a bad trail is illegal one byte wide and a lead at
# end of input is incomplete.
try:
    bytes([0xa1, 0x20]).decode("cp949")
except UnicodeDecodeError as e:
    print("decode bad trail:", e)
try:
    bytes([0x41, 0xa1]).decode("cp949")
except UnicodeDecodeError as e:
    print("decode incomplete:", e)
bad = bytes([0xa1, 0x20]) + "한".encode("cp949")
print("decode ignore eq:", bad.decode("cp949", "ignore") == " 한")
print("decode replace eq:", bad.decode("cp949", "replace") == "� 한")

# Encode error handling. An emoji outside the BMP has no cp949 mapping.
try:
    "a\U0001f600b".encode("cp949")
except UnicodeEncodeError as e:
    print("encode strict:", e)
print("encode ignore eq:", "a\U0001f600b".encode("cp949", "ignore") == b"ab")
print("encode replace eq:", "a\U0001f600b".encode("cp949", "replace") == b"a?b")
