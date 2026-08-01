# big5hkscs on the _multibytecodec engine, driven through the vendored
# encodings/big5hkscs.py. big5hkscs is the Hong Kong Supplementary Character Set
# on top of big5: a fixed-width two-byte codec whose double space also carries
# supplementary-plane characters and four combining sequences that decode to a
# base (U+00CA or U+00EA) plus a combining mark, and the encoder folds a base and
# its mark back into that one unit. The text below mixes big5 kanji, a combining
# pair (Ê̄, U+00CA U+0304), and a supplementary-plane character (U+233B4). This
# exercises the stateless encode/decode, the incremental encoder (including the
# combining base held across a chunk boundary) and decoder, the stream reader and
# writer, and the strict/ignore/replace error handling, all of which must match
# CPython byte for byte.
import codecs
import io

text = "中文 abc 香港 Ê̄ \U000233b4 測試"

data = text.encode("big5hkscs")
print("encoded hex:", data.hex())
print("decoded eq:", data.decode("big5hkscs") == text)
print("codecs.encode eq:", codecs.encode(text, "big5hkscs") == data)
print("codecs.decode eq:", codecs.decode(data, "big5hkscs") == text)

info = codecs.lookup("big5hkscs")
print("info name:", info.name)
print("info encode eq:", info.encode(text)[0] == data)
print("info decode eq:", info.decode(data)[0] == text)

# The combining pair Ê̄ (U+00CA U+0304) is one two-byte unit; encoding it whole
# and encoding the base and mark separately must produce the same bytes.
pair = "Ê̄"
print("pair hex:", pair.encode("big5hkscs").hex())
print("pair roundtrip:", pair.encode("big5hkscs").decode("big5hkscs") == pair)
print("base alone hex:", "Ê".encode("big5hkscs").hex())
print("supp hex:", "\U000233b4".encode("big5hkscs").hex())

# The incremental encoder holds the combining base until the mark arrives, so the
# base emits nothing on its own chunk and the pair lands on the mark's chunk.
enc = codecs.getincrementalencoder("big5hkscs")()
first = enc.encode("Ê")
rest = enc.encode("̄", True)
print("base chunk empty:", first == b"")
print("pair on mark chunk:", (first + rest) == pair.encode("big5hkscs"))

enc2 = codecs.getincrementalencoder("big5hkscs")()
chunks = b""
for ch in text:
    chunks += enc2.encode(ch)
chunks += enc2.encode("", True)
print("incremental encode eq:", chunks == data)

dec = codecs.getincrementaldecoder("big5hkscs")()
out = ""
for i in range(0, len(data), 3):
    out += dec.decode(data[i:i + 3], False)
out += dec.decode(b"", True)
print("incremental decode eq:", out == text)

buf = io.BytesIO()
writer = codecs.getwriter("big5hkscs")(buf)
writer.write(text)
writer.flush()
print("stream bytes eq:", buf.getvalue() == data)
buf.seek(0)
reader = codecs.getreader("big5hkscs")(buf)
print("stream read eq:", reader.read() == text)

# Decode error handling. A lead byte with a bad trail is illegal one byte wide,
# and a lone lead byte at the end is incomplete.
try:
    bytes([0x81, 0x20]).decode("big5hkscs")
except UnicodeDecodeError as e:
    print("decode bad trail:", e)
try:
    bytes([0x41, 0x81]).decode("big5hkscs")
except UnicodeDecodeError as e:
    print("decode incomplete:", e)
bad = bytes([0x81, 0x20]) + "中".encode("big5hkscs")
print("decode ignore eq:", bad.decode("big5hkscs", "ignore") == " 中")
print("decode replace eq:", bad.decode("big5hkscs", "replace") == "� 中")

# Encode error handling. The emoji U+1F600 has no big5hkscs mapping.
try:
    "a\U0001F600b".encode("big5hkscs")
except UnicodeEncodeError as e:
    print("encode strict:", e)
print("encode ignore eq:", "a\U0001F600b".encode("big5hkscs", "ignore") == b"ab")
print("encode replace eq:", "a\U0001F600b".encode("big5hkscs", "replace") == b"a?b")
