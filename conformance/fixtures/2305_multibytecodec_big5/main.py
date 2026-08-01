# The _multibytecodec engine driving big5 through the vendored encodings/big5.py.
# big5 is the Taiwanese standard, a fixed-width two-byte codec: ascii passes
# through and a lead byte with a trail selects one BMP code point. This exercises
# the stateless encode/decode, the incremental encoder and decoder (including a
# sequence split across a chunk boundary), the stream reader and writer, and the
# strict/ignore/replace error handling, all of which must match CPython byte for
# byte.
import codecs
import io

text = "中文 abc 你好 台灣 繁體字測試"

# Stateless roundtrip through str.encode / bytes.decode and codecs.encode/decode.
data = text.encode("big5")
print("encoded hex:", data.hex())
print("decoded eq:", data.decode("big5") == text)
print("codecs.encode eq:", codecs.encode(text, "big5") == data)
print("codecs.decode eq:", codecs.decode(data, "big5") == text)

# codecs.lookup exposes the CodecInfo the encodings module builds.
info = codecs.lookup("big5")
print("info name:", info.name)
print("info encode eq:", info.encode(text)[0] == data)
print("info decode eq:", info.decode(data)[0] == text)

# Incremental encoder: feeding one character at a time yields the same bytes.
enc = codecs.getincrementalencoder("big5")()
chunks = b""
for ch in text:
    chunks += enc.encode(ch)
chunks += enc.encode("", True)
print("incremental encode eq:", chunks == data)

# Incremental decoder over arbitrary byte chunks, including a split that lands in
# the middle of a double-byte character, must reassemble the whole string.
dec = codecs.getincrementaldecoder("big5")()
out = ""
step = 3
for i in range(0, len(data), step):
    out += dec.decode(data[i:i + step], False)
out += dec.decode(b"", True)
print("incremental decode eq:", out == text)

# A lead byte with no trailing byte is buffered until the next chunk completes it.
dec2 = codecs.getincrementaldecoder("big5")()
print("buffered lead:", repr(dec2.decode(bytes([0xA4]), False)))
print("completed pair:", dec2.decode(bytes([0xA4]), True) == "中")

# Stream writer and reader roundtrip over an in-memory byte stream.
buf = io.BytesIO()
writer = codecs.getwriter("big5")(buf)
writer.write(text)
writer.flush()
print("stream bytes eq:", buf.getvalue() == data)
buf.seek(0)
reader = codecs.getreader("big5")(buf)
print("stream read eq:", reader.read() == text)

# Error handling on decode: strict raises, ignore drops the bad span, replace
# emits U+FFFD. big5 reports each bad or incomplete sequence one byte wide.
bad = bytes([0x81, 0x20, 0xA4, 0xA4])
try:
    bad.decode("big5")
except UnicodeDecodeError as e:
    print("decode strict:", e)
print("decode ignore eq:", bad.decode("big5", "ignore") == " 中")
print("decode replace eq:", bad.decode("big5", "replace") == "� 中")

incomplete = bytes([0x41, 0x81])
try:
    incomplete.decode("big5")
except UnicodeDecodeError as e:
    print("decode incomplete:", e)

# Error handling on encode: strict raises, ignore drops, replace emits '?'.
try:
    "aÿb".encode("big5")
except UnicodeEncodeError as e:
    print("encode strict:", e)
print("encode ignore eq:", "aÿb".encode("big5", "ignore") == b"ab")
print("encode replace eq:", "aÿb".encode("big5", "replace") == b"a?b")
