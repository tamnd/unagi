# cp932 on the _multibytecodec engine, Microsoft's shift_jis superset, driven
# through the vendored encodings/cp932.py. cp932 decodes ascii and half-width
# katakana as single bytes and adds the NEC and IBM extension rows on top of the
# shift_jis two-byte space, so glyphs like the circled digits and the parenthesised
# company mark that plain shift_jis rejects encode here. This exercises the
# stateless encode/decode, the incremental encoder and decoder (including a
# character split across a chunk boundary), the stream reader and writer, and the
# strict/ignore/replace error handling, all of which must match CPython byte for
# byte.
import codecs
import io

text = "日本語 abc ｱｲｳ 123 ①②③ ㈱ テスト。漢字ひらがな"

data = text.encode("cp932")
print("encoded hex:", data.hex())
print("decoded eq:", data.decode("cp932") == text)
print("codecs.encode eq:", codecs.encode(text, "cp932") == data)
print("codecs.decode eq:", codecs.decode(data, "cp932") == text)

info = codecs.lookup("cp932")
print("info name:", info.name)
print("info encode eq:", info.encode(text)[0] == data)
print("info decode eq:", info.decode(data)[0] == text)

enc = codecs.getincrementalencoder("cp932")()
chunks = b""
for ch in text:
    chunks += enc.encode(ch)
chunks += enc.encode("", True)
print("incremental encode eq:", chunks == data)

dec = codecs.getincrementaldecoder("cp932")()
out = ""
step = 3
for i in range(0, len(data), step):
    out += dec.decode(data[i:i + step], False)
out += dec.decode(b"", True)
print("incremental decode eq:", out == text)

# A lead byte with no trailing byte buffers until the next chunk completes it.
dec2 = codecs.getincrementaldecoder("cp932")()
lead = "①".encode("cp932")
print("buffered lead:", repr(dec2.decode(lead[:1], False)))
print("completed pair:", dec2.decode(lead[1:], True) == "①")

buf = io.BytesIO()
writer = codecs.getwriter("cp932")(buf)
writer.write(text)
writer.flush()
print("stream bytes eq:", buf.getvalue() == data)
buf.seek(0)
reader = codecs.getreader("cp932")(buf)
print("stream read eq:", reader.read() == text)

# Decode error handling. cp932 has no illegal standalone byte, so a lead byte
# in a range with no assigned character is illegal one byte wide once a trail
# follows, and a lead at end of input is incomplete.
try:
    bytes([0x85, 0x40]).decode("cp932")
except UnicodeDecodeError as e:
    print("decode bad lead:", e)
try:
    bytes([0x81, 0x20]).decode("cp932")
except UnicodeDecodeError as e:
    print("decode bad trail:", e)
try:
    bytes([0x41, 0x81]).decode("cp932")
except UnicodeDecodeError as e:
    print("decode incomplete:", e)
bad = bytes([0x85, 0x40]) + "あ".encode("cp932")
print("decode ignore eq:", bad.decode("cp932", "ignore") == "@あ")
print("decode replace eq:", bad.decode("cp932", "replace") == "�@あ")

# Encode error handling. The euro sign has no cp932 mapping.
try:
    "a€b".encode("cp932")
except UnicodeEncodeError as e:
    print("encode strict:", e)
print("encode ignore eq:", "a€b".encode("cp932", "ignore") == b"ab")
print("encode replace eq:", "a€b".encode("cp932", "replace") == b"a?b")
