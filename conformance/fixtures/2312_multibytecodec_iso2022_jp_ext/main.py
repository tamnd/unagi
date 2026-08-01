# iso2022_jp_ext on the _multibytecodec engine, driven through the vendored
# encodings/iso2022_jp_ext.py. iso2022_jp_ext is iso2022_jp plus JIS X 0212 (the
# same four-byte ESC$(D as iso2022_jp_1) and JIS X 0201 katakana, designated with
# the single-byte escape ESC(I. The katakana charset is the first single-byte
# designated charset on the shared ISO-2022 machine: its GL byte 0x21..0x5f maps
# linearly to halfwidth katakana U+FF61..U+FF9F, and a byte outside that range is
# illegal one byte wide. Everything else (ascii, roman, JIS X 0208, and ESC$@ for
# the 1978 revision on decode) is shared with the base codec. The getstate int packs
# the katakana designation code (0x49) into the low byte the same way it packs the
# two-byte charsets, and all of it must match CPython byte for byte.
import codecs
import io

# The text mixes ascii, halfwidth katakana, a JIS X 0208 kanji, a JIS X 0212
# character, the yen roman special and a trailing ascii run, so the encoder opens
# and closes ESC(I, ESC$B, ESC$(D and ESC(J designations.
text = "AB ｱｶ 漢 丨 ¥ x"

data = text.encode("iso2022_jp_ext")
print("encoded hex:", data.hex())
print("decoded eq:", data.decode("iso2022_jp_ext") == text)
print("codecs.encode eq:", codecs.encode(text, "iso2022_jp_ext") == data)
print("codecs.decode eq:", codecs.decode(data, "iso2022_jp_ext") == text)

# Halfwidth katakana designates the single-byte ESC(I; a kanji still uses ESC$B and
# a JIS X 0212 character still uses ESC$(D.
print("kana hex:", "ｱ".encode("iso2022_jp_ext").hex())
print("jisx0208 hex:", "漢".encode("iso2022_jp_ext").hex())
print("jisx0212 hex:", "丨".encode("iso2022_jp_ext").hex())

info = codecs.lookup("iso2022_jp_ext")
print("info name:", info.name)
print("info encode eq:", info.encode(text)[0] == data)
print("info decode eq:", info.decode(data)[0] == text)

# Incremental encoder over one character at a time yields the same bytes.
enc = codecs.getincrementalencoder("iso2022_jp_ext")()
chunks = b""
for ch in text:
    chunks += enc.encode(ch)
chunks += enc.encode("", True)
print("incremental encode eq:", chunks == data)

# getstate packs the single-byte katakana designation into the low byte.
enc2 = codecs.getincrementalencoder("iso2022_jp_ext")()
enc2.encode("ｱ")
print("enc state kana:", enc2.getstate())

# Incremental decoder over three-byte chunks, one of which splits the ESC(I escape,
# reassembles the whole string.
dec = codecs.getincrementaldecoder("iso2022_jp_ext")()
out = ""
step = 3
for i in range(0, len(data), step):
    out += dec.decode(data[i:i + step], False)
out += dec.decode(b"", True)
print("incremental decode eq:", out == text)

# The decoder carries the single-byte designation across a chunk boundary: feed a
# partial ESC( then complete it and a kana byte in the next chunk.
dec2 = codecs.getincrementaldecoder("iso2022_jp_ext")()
print("buffered partial:", repr(dec2.decode(b"a\x1b(", False)))
print("decoder state:", dec2.getstate())
print("completed kana:", dec2.decode(b"I1", True))

# getstate/setstate roundtrip mid katakana mode.
dec3 = codecs.getincrementaldecoder("iso2022_jp_ext")()
dec3.decode(b"\x1b(I", False)
state = dec3.getstate()
dec4 = codecs.getincrementaldecoder("iso2022_jp_ext")()
dec4.setstate(state)
print("setstate resumes kana:", dec4.decode(b"1\x1b(B", True))

# Stream writer and reader roundtrip.
buf = io.BytesIO()
writer = codecs.getwriter("iso2022_jp_ext")(buf)
writer.write(text)
writer.flush()
buf.seek(0)
reader = codecs.getreader("iso2022_jp_ext")(buf)
print("stream read eq:", reader.read() == text)

# A katakana byte outside 0x21..0x5f is illegal one byte wide; strict raises, ignore
# drops it, replace emits U+FFFD.
bad = b"\x1b(I\x60\x1b(B"
try:
    bad.decode("iso2022_jp_ext")
except UnicodeDecodeError as e:
    print("decode strict:", e)
print("decode ignore:", repr(bad.decode("iso2022_jp_ext", "ignore")))
print("decode replace:", repr(bad.decode("iso2022_jp_ext", "replace")))

# Encode error handling: a code point iso2022_jp_ext cannot represent raises under
# strict and emits '?' under replace, returning to ascii first.
try:
    "\U0001F600".encode("iso2022_jp_ext")
except UnicodeEncodeError as e:
    print("encode strict:", e)
print("encode replace hex:", ("ｱ" + "\U0001F600").encode("iso2022_jp_ext", "replace").hex())
