# shift_jis on the _multibytecodec engine, the first Japanese codec, driven
# through the vendored encodings/shift_jis.py. shift_jis decodes ascii and
# half-width katakana as single bytes and a lead 0x81..0x9F or 0xE0..0xEA plus a
# trail as a two-byte character. This exercises the stateless encode/decode, the
# incremental encoder and decoder (including a character split across a chunk
# boundary), the stream reader and writer, and the strict/ignore/replace error
# handling, all of which must match CPython byte for byte.
import codecs
import io

text = "日本語 abc ｱｲｳ カタカナ 123 テスト。漢字ひらがな"

data = text.encode("shift_jis")
print("encoded hex:", data.hex())
print("decoded eq:", data.decode("shift_jis") == text)
print("codecs.encode eq:", codecs.encode(text, "shift_jis") == data)
print("codecs.decode eq:", codecs.decode(data, "shift_jis") == text)

info = codecs.lookup("shift_jis")
print("info name:", info.name)
print("info encode eq:", info.encode(text)[0] == data)
print("info decode eq:", info.decode(data)[0] == text)

enc = codecs.getincrementalencoder("shift_jis")()
chunks = b""
for ch in text:
    chunks += enc.encode(ch)
chunks += enc.encode("", True)
print("incremental encode eq:", chunks == data)

dec = codecs.getincrementaldecoder("shift_jis")()
out = ""
step = 3
for i in range(0, len(data), step):
    out += dec.decode(data[i:i + step], False)
out += dec.decode(b"", True)
print("incremental decode eq:", out == text)

# A lead byte with no trailing byte buffers until the next chunk completes it.
dec2 = codecs.getincrementaldecoder("shift_jis")()
lead = "日".encode("shift_jis")
print("buffered lead:", repr(dec2.decode(lead[:1], False)))
print("completed pair:", dec2.decode(lead[1:], True) == "日")

buf = io.BytesIO()
writer = codecs.getwriter("shift_jis")(buf)
writer.write(text)
writer.flush()
print("stream bytes eq:", buf.getvalue() == data)
buf.seek(0)
reader = codecs.getreader("shift_jis")(buf)
print("stream read eq:", reader.read() == text)

# Decode error handling. 0x80 is illegal on its own; a valid lead with a bad
# trail is illegal one byte wide; a lead at end of input is incomplete.
try:
    bytes([0x80]).decode("shift_jis")
except UnicodeDecodeError as e:
    print("decode illegal:", e)
try:
    bytes([0x81, 0x20]).decode("shift_jis")
except UnicodeDecodeError as e:
    print("decode bad trail:", e)
try:
    bytes([0x41, 0x81]).decode("shift_jis")
except UnicodeDecodeError as e:
    print("decode incomplete:", e)
bad = bytes([0x81, 0x20]) + "あ".encode("shift_jis")
print("decode ignore eq:", bad.decode("shift_jis", "ignore") == " あ")
print("decode replace eq:", bad.decode("shift_jis", "replace") == "� あ")

# Encode error handling. The euro sign has no shift_jis mapping.
try:
    "a€b".encode("shift_jis")
except UnicodeEncodeError as e:
    print("encode strict:", e)
print("encode ignore eq:", "a€b".encode("shift_jis", "ignore") == b"ab")
print("encode replace eq:", "a€b".encode("shift_jis", "replace") == b"a?b")
