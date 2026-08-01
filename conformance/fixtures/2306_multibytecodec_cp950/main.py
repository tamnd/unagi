# The _multibytecodec engine driving cp950 through the vendored encodings/cp950.py.
# cp950 is Microsoft's big5 variant, a fixed-width two-byte codec: ascii passes
# through and a lead byte with a trail selects one BMP code point, with a few
# extra rows big5 does not carry. This exercises the stateless encode/decode, the
# incremental encoder and decoder (including a sequence split across a chunk
# boundary), the stream reader and writer, and the strict/ignore/replace error
# handling, all of which must match CPython byte for byte.
import codecs
import io

# € (U+20AC) is in cp950 but not big5, so this string only encodes under the
# Microsoft variant.
text = "中文 abc 你好 台灣 €符號"

# Stateless roundtrip through str.encode / bytes.decode and codecs.encode/decode.
data = text.encode("cp950")
print("encoded hex:", data.hex())
print("decoded eq:", data.decode("cp950") == text)
print("codecs.encode eq:", codecs.encode(text, "cp950") == data)
print("codecs.decode eq:", codecs.decode(data, "cp950") == text)

# cp950 carries rows big5 does not: the euro sign does not encode under big5.
try:
    "€".encode("big5")
except UnicodeEncodeError as e:
    print("big5 rejects cp950-only:", e)
print("cp950-only hex:", "€".encode("cp950").hex())

# codecs.lookup exposes the CodecInfo the encodings module builds.
info = codecs.lookup("cp950")
print("info name:", info.name)
print("info encode eq:", info.encode(text)[0] == data)
print("info decode eq:", info.decode(data)[0] == text)

# Incremental encoder: feeding one character at a time yields the same bytes.
enc = codecs.getincrementalencoder("cp950")()
chunks = b""
for ch in text:
    chunks += enc.encode(ch)
chunks += enc.encode("", True)
print("incremental encode eq:", chunks == data)

# Incremental decoder over arbitrary byte chunks, including a split that lands in
# the middle of a double-byte character, must reassemble the whole string.
dec = codecs.getincrementaldecoder("cp950")()
out = ""
step = 3
for i in range(0, len(data), step):
    out += dec.decode(data[i:i + step], False)
out += dec.decode(b"", True)
print("incremental decode eq:", out == text)

# A lead byte with no trailing byte is buffered until the next chunk completes it.
dec2 = codecs.getincrementaldecoder("cp950")()
print("buffered lead:", repr(dec2.decode(bytes([0xA4]), False)))
print("completed pair:", dec2.decode(bytes([0xA4]), True) == "中")

# Stream writer and reader roundtrip over an in-memory byte stream.
buf = io.BytesIO()
writer = codecs.getwriter("cp950")(buf)
writer.write(text)
writer.flush()
print("stream bytes eq:", buf.getvalue() == data)
buf.seek(0)
reader = codecs.getreader("cp950")(buf)
print("stream read eq:", reader.read() == text)

# Error handling on decode: strict raises, ignore drops the bad span, replace
# emits U+FFFD. cp950 reports each bad or incomplete sequence one byte wide.
bad = bytes([0x81, 0x20, 0xA4, 0xA4])
try:
    bad.decode("cp950")
except UnicodeDecodeError as e:
    print("decode strict:", e)
print("decode ignore eq:", bad.decode("cp950", "ignore") == " 中")
print("decode replace eq:", bad.decode("cp950", "replace") == "� 中")

incomplete = bytes([0x41, 0x81])
try:
    incomplete.decode("cp950")
except UnicodeDecodeError as e:
    print("decode incomplete:", e)

# Error handling on encode: strict raises, ignore drops, replace emits '?'.
try:
    "aÿb".encode("cp950")
except UnicodeEncodeError as e:
    print("encode strict:", e)
print("encode ignore eq:", "aÿb".encode("cp950", "ignore") == b"ab")
print("encode replace eq:", "aÿb".encode("cp950", "replace") == b"a?b")
