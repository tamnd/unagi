# gb18030 on the _multibytecodec engine, driven through the vendored
# encodings/gb18030.py. gb18030 is the full-Unicode member of the Chinese codec
# family: ascii single bytes, gbk-style two-byte pairs, and an algorithmic
# four-byte form (a lead, a digit, a lead, a digit) whose linear index maps every
# remaining code point, including the whole supplementary plane. The text below
# mixes two-byte characters, a four-byte BMP character, an emoji and a
# supplementary-plane character. This exercises the stateless encode/decode, the
# incremental encoder and decoder (including a sequence split across a chunk
# boundary), the stream reader and writer, and the strict/ignore/replace error
# handling, all of which must match CPython byte for byte.
import codecs
import io

text = "汉字 abc 你好世界 \U0001F600 \U000233b4 符号"

# Stateless roundtrip through str.encode / bytes.decode and codecs.encode/decode.
data = text.encode("gb18030")
print("encoded hex:", data.hex())
print("decoded eq:", data.decode("gb18030") == text)
print("codecs.encode eq:", codecs.encode(text, "gb18030") == data)
print("codecs.decode eq:", codecs.decode(data, "gb18030") == text)

# gb18030 covers all of Unicode: the emoji that gbk cannot encode has a four-byte
# gb18030 form.
try:
    "\U0001F600".encode("gbk")
except UnicodeEncodeError as e:
    print("gbk rejects emoji:", e)
print("emoji four-byte hex:", "\U0001F600".encode("gb18030").hex())
print("supp four-byte hex:", "\U000233b4".encode("gb18030").hex())

# codecs.lookup exposes the CodecInfo the encodings module builds.
info = codecs.lookup("gb18030")
print("info name:", info.name)
print("info encode eq:", info.encode(text)[0] == data)
print("info decode eq:", info.decode(data)[0] == text)

# Incremental encoder: feeding one character at a time yields the same bytes.
enc = codecs.getincrementalencoder("gb18030")()
chunks = b""
for ch in text:
    chunks += enc.encode(ch)
chunks += enc.encode("", True)
print("incremental encode eq:", chunks == data)

# Incremental decoder over arbitrary byte chunks, including a split that lands in
# the middle of a four-byte character, must reassemble the whole string.
dec = codecs.getincrementaldecoder("gb18030")()
out = ""
step = 3
for i in range(0, len(data), step):
    out += dec.decode(data[i:i + step], False)
out += dec.decode(b"", True)
print("incremental decode eq:", out == text)

# A lead byte with only part of its four-byte sequence is buffered until the rest
# arrives.
dec2 = codecs.getincrementaldecoder("gb18030")()
print("buffered partial:", repr(dec2.decode(bytes([0x94, 0x39]), False)))
print("completed quad:", dec2.decode(bytes([0xFC, 0x36]), True) == "\U0001F600")

# Stream writer and reader roundtrip over an in-memory byte stream.
buf = io.BytesIO()
writer = codecs.getwriter("gb18030")(buf)
writer.write(text)
writer.flush()
print("stream bytes eq:", buf.getvalue() == data)
buf.seek(0)
reader = codecs.getreader("gb18030")(buf)
print("stream read eq:", reader.read() == text)

# Error handling on decode: strict raises, ignore drops the bad span, replace
# emits U+FFFD. A two-byte pair with a bad trail is illegal one byte wide.
bad = bytes([0x81, 0x20, 0xBA, 0xBA])
try:
    bad.decode("gb18030")
except UnicodeDecodeError as e:
    print("decode strict:", e)
print("decode ignore eq:", bad.decode("gb18030", "ignore") == " 汉")
print("decode replace eq:", bad.decode("gb18030", "replace") == "� 汉")

# A four-byte form with a bad fourth byte is illegal at the lead, and a truncated
# four-byte form is incomplete over the bytes in hand.
try:
    bytes([0x81, 0x30, 0x81, 0x20]).decode("gb18030")
except UnicodeDecodeError as e:
    print("decode bad quad:", e)
try:
    bytes([0x81, 0x30, 0x81]).decode("gb18030")
except UnicodeDecodeError as e:
    print("decode incomplete:", e)
