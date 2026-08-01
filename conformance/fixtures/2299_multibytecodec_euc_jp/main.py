# euc_jp on the _multibytecodec engine, the first variable-width Japanese codec,
# driven through the vendored encodings/euc_jp.py. euc_jp decodes ascii as single
# bytes, 0x8e (SS2) plus one byte as a half-width katakana, 0x8f (SS3) plus two
# bytes as a JIS X 0212 character, and any other high byte as a two-byte JIS X
# 0208 lead. The text below mixes kanji, half-width katakana (the 0x8e path) and
# a JIS X 0212 breve (the 0x8f three-byte path). This exercises the stateless
# encode/decode, the incremental encoder and decoder (including a three-byte
# character split across a chunk boundary), the stream reader and writer, and the
# strict/ignore/replace error handling, all of which must match CPython byte for
# byte.
import codecs
import io

text = "日本語 abc ｱｲｳ 123 ˘ テスト。漢字ひらがな"

data = text.encode("euc_jp")
print("encoded hex:", data.hex())
print("decoded eq:", data.decode("euc_jp") == text)
print("codecs.encode eq:", codecs.encode(text, "euc_jp") == data)
print("codecs.decode eq:", codecs.decode(data, "euc_jp") == text)

info = codecs.lookup("euc_jp")
print("info name:", info.name)
print("info encode eq:", info.encode(text)[0] == data)
print("info decode eq:", info.decode(data)[0] == text)

enc = codecs.getincrementalencoder("euc_jp")()
chunks = b""
for ch in text:
    chunks += enc.encode(ch)
chunks += enc.encode("", True)
print("incremental encode eq:", chunks == data)

dec = codecs.getincrementaldecoder("euc_jp")()
out = ""
step = 3
for i in range(0, len(data), step):
    out += dec.decode(data[i:i + step], False)
out += dec.decode(b"", True)
print("incremental decode eq:", out == text)

# The 0x8f three-byte sequence split one byte at a time buffers until complete.
dec2 = codecs.getincrementaldecoder("euc_jp")()
ss3 = "˘".encode("euc_jp")
print("ss3 len:", len(ss3))
print("after 1:", repr(dec2.decode(ss3[:1], False)))
print("after 2:", repr(dec2.decode(ss3[1:2], False)))
print("completed ss3:", dec2.decode(ss3[2:], True) == "˘")

buf = io.BytesIO()
writer = codecs.getwriter("euc_jp")(buf)
writer.write(text)
writer.flush()
print("stream bytes eq:", buf.getvalue() == data)
buf.seek(0)
reader = codecs.getreader("euc_jp")(buf)
print("stream read eq:", reader.read() == text)

# Decode error handling. Every high byte is a lead, so a lone high byte is
# incomplete; a two-byte lead with a bad trail is illegal one byte wide; the
# 0x8f single-shift wants two more bytes, so it reports the bytes in hand while
# it is short and resyncs one byte on when the completed sequence does not map.
try:
    bytes([0xa1, 0x20]).decode("euc_jp")
except UnicodeDecodeError as e:
    print("decode bad trail:", e)
try:
    bytes([0x41, 0xa1]).decode("euc_jp")
except UnicodeDecodeError as e:
    print("decode incomplete:", e)
try:
    bytes([0x8f, 0xa1]).decode("euc_jp")
except UnicodeDecodeError as e:
    print("decode ss3 short:", e)
try:
    bytes([0x8f, 0xa1, 0x20]).decode("euc_jp")
except UnicodeDecodeError as e:
    print("decode ss3 bad:", e)
bad = bytes([0xa1, 0x20]) + "あ".encode("euc_jp")
print("decode ignore eq:", bad.decode("euc_jp", "ignore") == " あ")
print("decode replace eq:", bad.decode("euc_jp", "replace") == "� あ")

# Encode error handling. The euro sign has no euc_jp mapping.
try:
    "a€b".encode("euc_jp")
except UnicodeEncodeError as e:
    print("encode strict:", e)
print("encode ignore eq:", "a€b".encode("euc_jp", "ignore") == b"ab")
print("encode replace eq:", "a€b".encode("euc_jp", "replace") == b"a?b")
