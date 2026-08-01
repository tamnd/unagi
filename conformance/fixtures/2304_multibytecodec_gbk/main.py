# The _multibytecodec engine driving gbk through the vendored encodings/gbk.py.
# gbk is a fixed-width two-byte codec, the full GBK repertoire on top of the
# gb2312 subset: ascii passes through and a lead byte with a trail selects one
# BMP code point. This exercises the stateless encode/decode, the incremental
# encoder and decoder (including a sequence split across a chunk boundary), the
# stream reader and writer, and the strict/ignore/replace error handling, all of
# which must match CPython byte for byte.
import codecs
import io

# 镕 (U+9555) is in gbk but not gb2312, so this string only encodes under the
# wider repertoire.
text = "汉字 abc 你好世界 【】符号 镕"

# Stateless roundtrip through str.encode / bytes.decode and codecs.encode/decode.
data = text.encode("gbk")
print("encoded hex:", data.hex())
print("decoded eq:", data.decode("gbk") == text)
print("codecs.encode eq:", codecs.encode(text, "gbk") == data)
print("codecs.decode eq:", codecs.decode(data, "gbk") == text)

# gbk is a strict superset of gb2312: the gbk-only character does not encode
# under gb2312.
try:
    "镕".encode("gb2312")
except UnicodeEncodeError as e:
    print("gb2312 rejects gbk-only:", e)
print("gbk-only hex:", "镕".encode("gbk").hex())

# codecs.lookup exposes the CodecInfo the encodings module builds.
info = codecs.lookup("gbk")
print("info name:", info.name)
print("info encode eq:", info.encode(text)[0] == data)
print("info decode eq:", info.decode(data)[0] == text)

# The '936', 'cp936' and 'ms936' aliases resolve to the same codec.
print("alias cp936:", codecs.lookup("cp936").name)

# Incremental encoder: feeding one character at a time yields the same bytes.
enc = codecs.getincrementalencoder("gbk")()
chunks = b""
for ch in text:
    chunks += enc.encode(ch)
chunks += enc.encode("", True)
print("incremental encode eq:", chunks == data)

# Incremental decoder over arbitrary byte chunks, including a split that lands in
# the middle of a double-byte character, must reassemble the whole string.
dec = codecs.getincrementaldecoder("gbk")()
out = ""
step = 3
for i in range(0, len(data), step):
    out += dec.decode(data[i:i + step], False)
out += dec.decode(b"", True)
print("incremental decode eq:", out == text)

# A lead byte with no trailing byte is buffered until the next chunk completes it.
dec2 = codecs.getincrementaldecoder("gbk")()
print("buffered lead:", repr(dec2.decode(bytes([0xBA]), False)))
print("completed pair:", dec2.decode(bytes([0xBA]), True) == "汉")

# Stream writer and reader roundtrip over an in-memory byte stream.
buf = io.BytesIO()
writer = codecs.getwriter("gbk")(buf)
writer.write(text)
writer.flush()
print("stream bytes eq:", buf.getvalue() == data)
buf.seek(0)
reader = codecs.getreader("gbk")(buf)
print("stream read eq:", reader.read() == text)

# Error handling on decode: strict raises, ignore drops the bad span, replace
# emits U+FFFD. gbk reports each bad or incomplete sequence one byte wide.
bad = bytes([0x81, 0x20, 0xBA, 0xBA])
try:
    bad.decode("gbk")
except UnicodeDecodeError as e:
    print("decode strict:", e)
print("decode ignore eq:", bad.decode("gbk", "ignore") == " 汉")
print("decode replace eq:", bad.decode("gbk", "replace") == "� 汉")

incomplete = bytes([0x41, 0x81])
try:
    incomplete.decode("gbk")
except UnicodeDecodeError as e:
    print("decode incomplete:", e)

# Error handling on encode: strict raises, ignore drops, replace emits '?'.
try:
    "aÿb".encode("gbk")
except UnicodeEncodeError as e:
    print("encode strict:", e)
print("encode ignore eq:", "aÿb".encode("gbk", "ignore") == b"ab")
print("encode replace eq:", "aÿb".encode("gbk", "replace") == b"a?b")
