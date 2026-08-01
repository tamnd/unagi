# euc_jis_2004 on the _multibytecodec engine, the JIS X 0213 member of the euc_jp
# family, driven through the vendored encodings/euc_jis_2004.py. It has the
# variable-width euc_jp byte structure (ascii single bytes, 0x8e plus one byte for
# half-width katakana, 0x8f plus two bytes for the plane 2 character, any other
# high byte a two-byte lead) but a few two-byte sequences in the double space
# decode to a base plus a combining mark, and the encoder folds a base and its
# mark back into that one sequence. The text below mixes kanji, half-width
# katakana (the 0x8e path), a plane 2 character (the 0x8f three-byte path), a
# combining pair (か゚, U+304B U+309A), and a supplementary-plane character
# (U+20089). This exercises the stateless encode/decode, the incremental encoder
# (including the combining base held across a chunk boundary) and decoder, the
# stream reader and writer, and the strict/ignore/replace error handling, all of
# which must match CPython byte for byte.
import codecs
import io

text = "日本語 abc ｱｲｳ か゚ \U00020089 テスト。"

data = text.encode("euc_jis_2004")
print("encoded hex:", data.hex())
print("decoded eq:", data.decode("euc_jis_2004") == text)
print("codecs.encode eq:", codecs.encode(text, "euc_jis_2004") == data)
print("codecs.decode eq:", codecs.decode(data, "euc_jis_2004") == text)

info = codecs.lookup("euc_jis_2004")
print("info name:", info.name)
print("info encode eq:", info.encode(text)[0] == data)
print("info decode eq:", info.decode(data)[0] == text)

# The combining pair か゚ (U+304B U+309A) is one two-byte unit; encoding it whole
# and encoding the base and mark separately must produce the same bytes.
pair = "か゚"
print("pair hex:", pair.encode("euc_jis_2004").hex())
print("pair roundtrip:", pair.encode("euc_jis_2004").decode("euc_jis_2004") == pair)
print("base alone hex:", "か".encode("euc_jis_2004").hex())
print("supp hex:", "\U00020089".encode("euc_jis_2004").hex())

# The incremental encoder holds the combining base until the mark arrives, so the
# base emits nothing on its own chunk and the pair lands on the mark's chunk.
enc = codecs.getincrementalencoder("euc_jis_2004")()
first = enc.encode("か")
rest = enc.encode("゚", True)
print("base chunk empty:", first == b"")
print("pair on mark chunk:", (first + rest) == pair.encode("euc_jis_2004"))

enc2 = codecs.getincrementalencoder("euc_jis_2004")()
chunks = b""
for ch in text:
    chunks += enc2.encode(ch)
chunks += enc2.encode("", True)
print("incremental encode eq:", chunks == data)

dec = codecs.getincrementaldecoder("euc_jis_2004")()
out = ""
for i in range(0, len(data), 3):
    out += dec.decode(data[i:i + 3], False)
out += dec.decode(b"", True)
print("incremental decode eq:", out == text)

buf = io.BytesIO()
writer = codecs.getwriter("euc_jis_2004")(buf)
writer.write(text)
writer.flush()
print("stream bytes eq:", buf.getvalue() == data)
buf.seek(0)
reader = codecs.getreader("euc_jis_2004")(buf)
print("stream read eq:", reader.read() == text)

# Decode error handling. Every high byte is a lead, so a lead with a bad trail is
# illegal one byte wide and a lone lead at the end is incomplete; the 0x8f
# single-shift wants two more bytes.
try:
    bytes([0xa1, 0x20]).decode("euc_jis_2004")
except UnicodeDecodeError as e:
    print("decode bad trail:", e)
try:
    bytes([0x41, 0xa1]).decode("euc_jis_2004")
except UnicodeDecodeError as e:
    print("decode incomplete:", e)
try:
    bytes([0x8f, 0xa1]).decode("euc_jis_2004")
except UnicodeDecodeError as e:
    print("decode ss3 short:", e)
bad = bytes([0xa1, 0x20]) + "あ".encode("euc_jis_2004")
print("decode ignore eq:", bad.decode("euc_jis_2004", "ignore") == " あ")
print("decode replace eq:", bad.decode("euc_jis_2004", "replace") == "� あ")

# Encode error handling. The emoji U+1F600 has no euc_jis_2004 mapping.
try:
    "a\U0001F600b".encode("euc_jis_2004")
except UnicodeEncodeError as e:
    print("encode strict:", e)
print("encode ignore eq:", "a\U0001F600b".encode("euc_jis_2004", "ignore") == b"ab")
print("encode replace eq:", "a\U0001F600b".encode("euc_jis_2004", "replace") == b"a?b")
