# iso2022_jp_2 on the _multibytecodec engine, driven through the vendored
# encodings/iso2022_jp_2.py. iso2022_jp_2 is the widest member of the ISO-2022-JP
# family: on top of the iso2022_jp base (ascii, JIS X 0201 roman ESC(J, JIS X 0208
# ESC$B) it adds JIS X 0212 (ESC$(D), GB 2312 (ESC$A or ESC$(A) and KSC 5601
# (ESC$(C) as two-byte G0 charsets, plus two decode-only single-byte G2 sets invoked
# with SS2 (ESC N): iso8859-1 (designated ESC.A) and iso8859-7 (ESC.F).
#
# The encoder tries the G0 charsets in the order ascii, roman, JIS X 0208, JIS X
# 0212, GB 2312, KSC 5601, so a character in more than one is emitted through the
# earliest, and it never emits SS2. The decoder carries a G2 designation alongside
# the G0 one, and getstate packs the G2 code into the bits above the G0 byte. All of
# it must match CPython byte for byte.
import codecs
import io

# The text mixes ascii, a JIS X 0208 kanji, a JIS X 0212 character, a GB 2312
# character and a KSC 5601 character, so the encoder walks the whole G0 order.
text = "AB 漢 丨 专 乫 x"

data = text.encode("iso2022_jp_2")
print("encoded hex:", data.hex())
print("decoded eq:", data.decode("iso2022_jp_2") == text)
print("codecs.encode eq:", codecs.encode(text, "iso2022_jp_2") == data)
print("codecs.decode eq:", codecs.decode(data, "iso2022_jp_2") == text)

# Each subcharset designates its own escape.
print("jisx0208 hex:", "漢".encode("iso2022_jp_2").hex())
print("jisx0212 hex:", "丨".encode("iso2022_jp_2").hex())
print("gb2312 hex:", "专".encode("iso2022_jp_2").hex())
print("ksc5601 hex:", "乫".encode("iso2022_jp_2").hex())
print("roman yen hex:", "¥".encode("iso2022_jp_2").hex())

# The decode-only G2 sets come back through SS2. The decoder also accepts the short
# ESC$A form for GB 2312 alongside the long ESC$(A.
print("g2 latin1:", b"\x1b.A\x1bN\x69\x1b(B".decode("iso2022_jp_2"))
print("g2 latin7:", b"\x1b.F\x1bN\x61\x1b(B".decode("iso2022_jp_2"))
print("gb short form:", b"\x1b$AW(\x1b(B".decode("iso2022_jp_2"))

info = codecs.lookup("iso2022_jp_2")
print("info name:", info.name)
print("info encode eq:", info.encode(text)[0] == data)
print("info decode eq:", info.decode(data)[0] == text)

# Incremental encoder over one character at a time yields the same bytes.
enc = codecs.getincrementalencoder("iso2022_jp_2")()
chunks = b""
for ch in text:
    chunks += enc.encode(ch)
chunks += enc.encode("", True)
print("incremental encode eq:", chunks == data)

# getstate packs each G0 designation into the low byte.
enc2 = codecs.getincrementalencoder("iso2022_jp_2")()
enc2.encode("专")
print("enc state gb2312:", enc2.getstate())
enc3 = codecs.getincrementalencoder("iso2022_jp_2")()
enc3.encode("乫")
print("enc state ksc:", enc3.getstate())

# Incremental decoder over three-byte chunks reassembles the whole string.
dec = codecs.getincrementaldecoder("iso2022_jp_2")()
out = ""
step = 3
for i in range(0, len(data), step):
    out += dec.decode(data[i:i + step], False)
out += dec.decode(b"", True)
print("incremental decode eq:", out == text)

# The decoder carries a split G2 designation and its SS2 across a chunk boundary, and
# getstate reports the G2 code above the G0 byte.
dec2 = codecs.getincrementaldecoder("iso2022_jp_2")()
print("g2 designated partial:", repr(dec2.decode(b"\x1b.A", False)))
print("g2 decoder state:", dec2.getstate())
print("g2 completes:", dec2.decode(b"\x1bN\x69", True))

# Stream writer and reader roundtrip.
buf = io.BytesIO()
writer = codecs.getwriter("iso2022_jp_2")(buf)
writer.write(text)
writer.flush()
buf.seek(0)
reader = codecs.getreader("iso2022_jp_2")(buf)
print("stream read eq:", reader.read() == text)

# Error handling on decode: a GB 2312 pair with a bad trail is illegal two bytes
# wide; strict raises, ignore drops it, replace emits U+FFFD.
bad = b"\x1b$(A\x21\x20\x1b(B"
try:
    bad.decode("iso2022_jp_2")
except UnicodeDecodeError as e:
    print("decode strict:", e)
print("decode ignore:", repr(bad.decode("iso2022_jp_2", "ignore")))
print("decode replace:", repr(bad.decode("iso2022_jp_2", "replace")))

# Encode error handling: an unrepresentable code point raises under strict and emits
# '?' under replace, returning to ascii first.
try:
    "\U0001F600".encode("iso2022_jp_2")
except UnicodeEncodeError as e:
    print("encode strict:", e)
print("encode replace hex:", ("专" + "\U0001F600").encode("iso2022_jp_2", "replace").hex())
