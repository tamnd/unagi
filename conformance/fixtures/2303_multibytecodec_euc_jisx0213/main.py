# euc_jisx0213 on the _multibytecodec engine, the JIS X 0213:2000 sibling of
# euc_jis_2004, driven through the vendored encodings/euc_jisx0213.py. It has the
# same variable-width euc_jp byte structure with the same combining sequences over
# a table that differs from euc_jis_2004 by a handful of code points. The text
# below mixes kanji, half-width katakana (the 0x8e path), a plane 2 character (the
# 0x8f three-byte path), a combining pair (か゚, U+304B U+309A), and a
# supplementary-plane character (U+20089). This exercises the stateless
# encode/decode, the incremental encoder (including the combining base held across
# a chunk boundary) and decoder, the stream reader and writer, and the
# strict/ignore/replace error handling, all of which must match CPython byte for
# byte.
import codecs
import io

text = "日本語 abc ｱｲｳ か゚ \U00020089 テスト。"

data = text.encode("euc_jisx0213")
print("encoded hex:", data.hex())
print("decoded eq:", data.decode("euc_jisx0213") == text)
print("codecs.encode eq:", codecs.encode(text, "euc_jisx0213") == data)
print("codecs.decode eq:", codecs.decode(data, "euc_jisx0213") == text)

info = codecs.lookup("euc_jisx0213")
print("info name:", info.name)
print("info encode eq:", info.encode(text)[0] == data)
print("info decode eq:", info.decode(data)[0] == text)

# The combining pair か゚ (U+304B U+309A) is one two-byte unit; encoding it whole
# and encoding the base and mark separately must produce the same bytes.
pair = "か゚"
print("pair hex:", pair.encode("euc_jisx0213").hex())
print("pair roundtrip:", pair.encode("euc_jisx0213").decode("euc_jisx0213") == pair)
print("base alone hex:", "か".encode("euc_jisx0213").hex())
print("supp hex:", "\U00020089".encode("euc_jisx0213").hex())

# The incremental encoder holds the combining base until the mark arrives, so the
# base emits nothing on its own chunk and the pair lands on the mark's chunk.
enc = codecs.getincrementalencoder("euc_jisx0213")()
first = enc.encode("か")
rest = enc.encode("゚", True)
print("base chunk empty:", first == b"")
print("pair on mark chunk:", (first + rest) == pair.encode("euc_jisx0213"))

enc2 = codecs.getincrementalencoder("euc_jisx0213")()
chunks = b""
for ch in text:
    chunks += enc2.encode(ch)
chunks += enc2.encode("", True)
print("incremental encode eq:", chunks == data)

dec = codecs.getincrementaldecoder("euc_jisx0213")()
out = ""
for i in range(0, len(data), 3):
    out += dec.decode(data[i:i + 3], False)
out += dec.decode(b"", True)
print("incremental decode eq:", out == text)

buf = io.BytesIO()
writer = codecs.getwriter("euc_jisx0213")(buf)
writer.write(text)
writer.flush()
print("stream bytes eq:", buf.getvalue() == data)
buf.seek(0)
reader = codecs.getreader("euc_jisx0213")(buf)
print("stream read eq:", reader.read() == text)

# Decode error handling. Every high byte is a lead, so a lead with a bad trail is
# illegal one byte wide and a lone lead at the end is incomplete; the 0x8f
# single-shift wants two more bytes.
try:
    bytes([0xa1, 0x20]).decode("euc_jisx0213")
except UnicodeDecodeError as e:
    print("decode bad trail:", e)
try:
    bytes([0x41, 0xa1]).decode("euc_jisx0213")
except UnicodeDecodeError as e:
    print("decode incomplete:", e)
try:
    bytes([0x8f, 0xa1]).decode("euc_jisx0213")
except UnicodeDecodeError as e:
    print("decode ss3 short:", e)
bad = bytes([0xa1, 0x20]) + "あ".encode("euc_jisx0213")
print("decode ignore eq:", bad.decode("euc_jisx0213", "ignore") == " あ")
print("decode replace eq:", bad.decode("euc_jisx0213", "replace") == "� あ")

# Encode error handling. The emoji U+1F600 has no euc_jisx0213 mapping.
try:
    "a\U0001F600b".encode("euc_jisx0213")
except UnicodeEncodeError as e:
    print("encode strict:", e)
print("encode ignore eq:", "a\U0001F600b".encode("euc_jisx0213", "ignore") == b"ab")
print("encode replace eq:", "a\U0001F600b".encode("euc_jisx0213", "replace") == b"a?b")
