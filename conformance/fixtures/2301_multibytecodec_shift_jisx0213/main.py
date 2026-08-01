# shift_jisx0213 on the _multibytecodec engine, the JIS X 0213:2000 sibling of
# shift_jis_2004, driven through the vendored encodings/shift_jisx0213.py. It has
# the same fixed-width shift_jis byte structure with the same combining sequences
# over a table that differs from shift_jis_2004 by a handful of code points. The
# text below mixes kanji, half-width katakana, a combining pair (か゚, U+304B
# U+309A), and a supplementary-plane character (U+20089). This exercises the
# stateless encode/decode, the incremental encoder (including the combining base
# held across a chunk boundary) and decoder, the stream reader and writer, and
# the strict/ignore/replace error handling, all of which must match CPython byte
# for byte.
import codecs
import io

text = "日本語 abc ｱｲｳ か゚ \U00020089 テスト。"

data = text.encode("shift_jisx0213")
print("encoded hex:", data.hex())
print("decoded eq:", data.decode("shift_jisx0213") == text)
print("codecs.encode eq:", codecs.encode(text, "shift_jisx0213") == data)
print("codecs.decode eq:", codecs.decode(data, "shift_jisx0213") == text)

info = codecs.lookup("shift_jisx0213")
print("info name:", info.name)
print("info encode eq:", info.encode(text)[0] == data)
print("info decode eq:", info.decode(data)[0] == text)

# The combining pair か゚ (U+304B U+309A) is one two-byte unit; encoding it whole
# and encoding the base and mark separately must produce the same bytes.
pair = "か゚"
print("pair hex:", pair.encode("shift_jisx0213").hex())
print("pair roundtrip:", pair.encode("shift_jisx0213").decode("shift_jisx0213") == pair)
print("base alone hex:", "か".encode("shift_jisx0213").hex())
print("supp hex:", "\U00020089".encode("shift_jisx0213").hex())

# The incremental encoder holds the combining base until the mark arrives, so the
# base emits nothing on its own chunk and the pair lands on the mark's chunk.
enc = codecs.getincrementalencoder("shift_jisx0213")()
first = enc.encode("か")
rest = enc.encode("゚", True)
print("base chunk empty:", first == b"")
print("pair on mark chunk:", (first + rest) == pair.encode("shift_jisx0213"))

enc2 = codecs.getincrementalencoder("shift_jisx0213")()
chunks = b""
for ch in text:
    chunks += enc2.encode(ch)
chunks += enc2.encode("", True)
print("incremental encode eq:", chunks == data)

dec = codecs.getincrementaldecoder("shift_jisx0213")()
out = ""
for i in range(0, len(data), 3):
    out += dec.decode(data[i:i + 3], False)
out += dec.decode(b"", True)
print("incremental decode eq:", out == text)

buf = io.BytesIO()
writer = codecs.getwriter("shift_jisx0213")(buf)
writer.write(text)
writer.flush()
print("stream bytes eq:", buf.getvalue() == data)
buf.seek(0)
reader = codecs.getreader("shift_jisx0213")(buf)
print("stream read eq:", reader.read() == text)

# Decode error handling. A lead byte with a bad trail is illegal one byte wide,
# and a lone lead byte at the end is incomplete.
try:
    bytes([0x81, 0x20]).decode("shift_jisx0213")
except UnicodeDecodeError as e:
    print("decode bad trail:", e)
try:
    bytes([0x41, 0x81]).decode("shift_jisx0213")
except UnicodeDecodeError as e:
    print("decode incomplete:", e)
bad = bytes([0x81, 0x20]) + "あ".encode("shift_jisx0213")
print("decode ignore eq:", bad.decode("shift_jisx0213", "ignore") == " あ")
print("decode replace eq:", bad.decode("shift_jisx0213", "replace") == "� あ")

# Encode error handling. The emoji U+1F600 has no shift_jisx0213 mapping.
try:
    "a\U0001F600b".encode("shift_jisx0213")
except UnicodeEncodeError as e:
    print("encode strict:", e)
print("encode ignore eq:", "a\U0001F600b".encode("shift_jisx0213", "ignore") == b"ab")
print("encode replace eq:", "a\U0001F600b".encode("shift_jisx0213", "replace") == b"a?b")
