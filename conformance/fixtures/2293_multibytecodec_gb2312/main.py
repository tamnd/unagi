# The _multibytecodec engine driving its first real codec, gb2312, through the
# vendored encodings/gb2312.py. gb2312 is a double-byte codec: ascii passes
# through and a lead 0xA1..0xF7 with a trail 0xA1..0xFE selects one BMP code
# point. This exercises the stateless encode/decode, the incremental encoder and
# decoder (including a sequence split across a chunk boundary), the stream
# reader and writer, and the strict/ignore/replace error handling, all of which
# must match CPython byte for byte.
import codecs
import io

text = "中文 abc 123 你好，世界！编码测试。"

# Stateless roundtrip through str.encode / bytes.decode and codecs.encode/decode.
data = text.encode("gb2312")
print("encoded hex:", data.hex())
print("decoded eq:", data.decode("gb2312") == text)
print("codecs.encode eq:", codecs.encode(text, "gb2312") == data)
print("codecs.decode eq:", codecs.decode(data, "gb2312") == text)

# codecs.lookup exposes the CodecInfo the encodings module builds.
info = codecs.lookup("gb2312")
print("info name:", info.name)
print("info encode eq:", info.encode(text)[0] == data)
print("info decode eq:", info.decode(data)[0] == text)

# Incremental encoder: feeding one character at a time yields the same bytes.
enc = codecs.getincrementalencoder("gb2312")()
chunks = b""
for ch in text:
    chunks += enc.encode(ch)
chunks += enc.encode("", True)
print("incremental encode eq:", chunks == data)

# Incremental decoder over arbitrary byte chunks, including a split that lands in
# the middle of a double-byte character, must reassemble the whole string.
dec = codecs.getincrementaldecoder("gb2312")()
out = ""
step = 3
for i in range(0, len(data), step):
    out += dec.decode(data[i:i + step], False)
out += dec.decode(b"", True)
print("incremental decode eq:", out == text)

# A lead byte with no trailing byte is buffered until the next chunk completes it.
dec2 = codecs.getincrementaldecoder("gb2312")()
print("buffered lead:", repr(dec2.decode(bytes([0xA1]), False)))
print("completed pair:", dec2.decode(bytes([0xA1]), True) == "　")

# Stream writer and reader roundtrip over an in-memory byte stream.
buf = io.BytesIO()
writer = codecs.getwriter("gb2312")(buf)
writer.write(text)
writer.flush()
print("stream bytes eq:", buf.getvalue() == data)
buf.seek(0)
reader = codecs.getreader("gb2312")(buf)
print("stream read eq:", reader.read() == text)

# Error handling on decode: strict raises, ignore drops the bad span, replace
# emits U+FFFD. gb2312 reports each bad or incomplete sequence one byte wide.
bad = bytes([0xA1, 0x20, 0xA1, 0xA1])
try:
    bad.decode("gb2312")
except UnicodeDecodeError as e:
    print("decode strict:", e)
print("decode ignore eq:", bad.decode("gb2312", "ignore") == " 　")
print("decode replace eq:", bad.decode("gb2312", "replace") == "� 　")

incomplete = bytes([0x41, 0xA1])
try:
    incomplete.decode("gb2312")
except UnicodeDecodeError as e:
    print("decode incomplete:", e)

# Error handling on encode: strict raises, ignore drops, replace emits '?'.
try:
    "aÿb".encode("gb2312")
except UnicodeEncodeError as e:
    print("encode strict:", e)
print("encode ignore eq:", "aÿb".encode("gb2312", "ignore") == b"ab")
print("encode replace eq:", "aÿb".encode("gb2312", "replace") == b"a?b")
