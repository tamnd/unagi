# iso2022_jp_1 on the _multibytecodec engine, driven through the vendored
# encodings/iso2022_jp_1.py. iso2022_jp_1 is iso2022_jp plus JIS X 0212, designated
# with the four-byte escape ESC$(D. Everything else (ascii ESC(B, JIS X 0201 roman
# ESC(J, JIS X 0208 ESC$B, and ESC$@ for the 1978 revision on decode) is shared with
# the base codec. This is the first variant on the shared ISO-2022 escape machine,
# so it exercises the config-driven charset repertoire: a character JIS X 0208 lacks
# designates ESC$(D, a character it has still designates ESC$B, and the decoder
# carries the extra designation across bytes and chunk boundaries. The getstate int
# packs the JIS X 0212 designation code (0xC4) into the low byte the same way the
# base codec packs 0208, and all of it must match CPython byte for byte.
import codecs
import io

# The text mixes ascii, a JIS X 0208 kanji, a JIS X 0212 character (U+4E28, not in
# 0208), the yen roman special and a trailing ascii run so the encoder opens and
# closes ESC$B, ESC$(D and ESC(J designations.
text = "ABC 漢 丨 ¥ x"

data = text.encode("iso2022_jp_1")
print("encoded hex:", data.hex())
print("decoded eq:", data.decode("iso2022_jp_1") == text)
print("codecs.encode eq:", codecs.encode(text, "iso2022_jp_1") == data)
print("codecs.decode eq:", codecs.decode(data, "iso2022_jp_1") == text)

# A JIS X 0212-only character designates ESC$(D; a JIS X 0208 kanji still uses ESC$B.
print("jisx0212 hex:", "丨".encode("iso2022_jp_1").hex())
print("jisx0208 hex:", "漢".encode("iso2022_jp_1").hex())

# The base-codec escapes still decode: ESC$@ is the 1978 revision, ESC(J roman.
print("jis1978 eq:", (b"\x1b$@4A\x1b(B").decode("iso2022_jp_1") == "漢")
print("roman yen eq:", (b"\x1b(J\x5c\x1b(B").decode("iso2022_jp_1") == "¥")

info = codecs.lookup("iso2022_jp_1")
print("info name:", info.name)
print("info encode eq:", info.encode(text)[0] == data)
print("info decode eq:", info.decode(data)[0] == text)

# Incremental encoder over one character at a time yields the same bytes.
enc = codecs.getincrementalencoder("iso2022_jp_1")()
chunks = b""
for ch in text:
    chunks += enc.encode(ch)
chunks += enc.encode("", True)
print("incremental encode eq:", chunks == data)

# getstate packs the JIS X 0212 designation into the low byte.
enc2 = codecs.getincrementalencoder("iso2022_jp_1")()
enc2.encode("丨")
print("enc state 0212:", enc2.getstate())

# Incremental decoder over three-byte chunks, one of which splits the ESC$(D escape,
# reassembles the whole string.
dec = codecs.getincrementaldecoder("iso2022_jp_1")()
out = ""
step = 3
for i in range(0, len(data), step):
    out += dec.decode(data[i:i + step], False)
out += dec.decode(b"", True)
print("incremental decode eq:", out == text)

# The decoder carries the four-byte designation across a chunk boundary: feed a
# partial ESC$( then complete it and a 0212 pair in the next chunk.
dec2 = codecs.getincrementaldecoder("iso2022_jp_1")()
print("buffered partial:", repr(dec2.decode(b"a\x1b$(", False)))
print("decoder state:", dec2.getstate())
print("completed 0212:", dec2.decode(b"D0)", True))

# getstate/setstate roundtrip mid JIS X 0212 mode.
dec3 = codecs.getincrementaldecoder("iso2022_jp_1")()
dec3.decode(b"\x1b$(D", False)
state = dec3.getstate()
dec4 = codecs.getincrementaldecoder("iso2022_jp_1")()
dec4.setstate(state)
print("setstate resumes 0212:", dec4.decode(b"0)\x1b(B", True))

# Stream writer and reader roundtrip.
buf = io.BytesIO()
writer = codecs.getwriter("iso2022_jp_1")(buf)
writer.write(text)
writer.flush()
buf.seek(0)
reader = codecs.getreader("iso2022_jp_1")(buf)
print("stream read eq:", reader.read() == text)

# Error handling on decode: a JIS X 0212 pair with a bad trail is illegal two bytes
# wide; strict raises, ignore drops it, replace emits U+FFFD.
bad = b"\x1b$(D\x21\x20\x1b(B"
try:
    bad.decode("iso2022_jp_1")
except UnicodeDecodeError as e:
    print("decode strict:", e)
print("decode ignore:", repr(bad.decode("iso2022_jp_1", "ignore")))
print("decode replace:", repr(bad.decode("iso2022_jp_1", "replace")))

# Encode error handling: a code point iso2022_jp_1 cannot represent raises under
# strict and emits '?' under replace, returning to ascii first.
try:
    "\U0001F600".encode("iso2022_jp_1")
except UnicodeEncodeError as e:
    print("encode strict:", e)
print("encode replace hex:", ("丨" + "\U0001F600").encode("iso2022_jp_1", "replace").hex())
