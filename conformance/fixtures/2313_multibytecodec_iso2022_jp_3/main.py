# iso2022_jp_3 on the _multibytecodec engine, driven through the vendored
# encodings/iso2022_jp_3.py. iso2022_jp_3 is a stricter member of the ISO-2022-JP
# family: it carries only ascii, JIS X 0208 (ESC$B) and the two JIS X 0213 planes,
# plane 1 by ESC$(O and plane 2 by ESC$(P. It does not designate JIS X 0201 roman,
# the 1978 revision, katakana or JIS X 0212, so the yen and overline specials the
# base codec folds into ESC(J are unencodable here.
#
# Plane 1 carries 25 combining sequences: a GL pair decodes to a base plus the
# combining mark U+309A, and the two-code-point sequence encodes back to that one
# pair. This is the first combining charset on the shared ISO-2022 machine, so it
# exercises the two-code-point decode, the pair encode tried ahead of the single
# encode, and the incremental encoder holding a combining base across a chunk
# boundary until its mark arrives. All of it must match CPython byte for byte.
import codecs
import io

# The text mixes ascii, a JIS X 0208 kanji, a plane 1 combining sequence (U+304B
# U+309A), a plane 1 single character and a trailing ascii run.
text = "AB 漢 か゚ 丨 x"

data = text.encode("iso2022_jp_3")
print("encoded hex:", data.hex())
print("decoded eq:", data.decode("iso2022_jp_3") == text)
print("codecs.encode eq:", codecs.encode(text, "iso2022_jp_3") == data)
print("codecs.decode eq:", codecs.decode(data, "iso2022_jp_3") == text)

# The combining sequence encodes as one plane 1 pair; a JIS X 0208 kanji still uses
# ESC$B.
print("combining hex:", "か゚".encode("iso2022_jp_3").hex())
print("jisx0208 hex:", "漢".encode("iso2022_jp_3").hex())
print("plane1 single hex:", "丨".encode("iso2022_jp_3").hex())

# A plane 2 pair decodes through ESC$(P.
print("plane2 decode:", b"\x1b$(P\x21\x21\x1b(B".decode("iso2022_jp_3"))

info = codecs.lookup("iso2022_jp_3")
print("info name:", info.name)
print("info encode eq:", info.encode(text)[0] == data)
print("info decode eq:", info.decode(data)[0] == text)

# Incremental encoder over one character at a time yields the same bytes. The base
# of the combining sequence is held until the mark arrives.
enc = codecs.getincrementalencoder("iso2022_jp_3")()
chunks = b""
for ch in text:
    chunks += enc.encode(ch)
chunks += enc.encode("", True)
print("incremental encode eq:", chunks == data)

# Feeding a combining base alone yields nothing; the following mark emits the pair.
enc2 = codecs.getincrementalencoder("iso2022_jp_3")()
print("base held:", enc2.encode("か").hex())
print("mark completes:", enc2.encode("゚", True).hex())

# getstate packs the plane 1 designation into the low byte.
enc3 = codecs.getincrementalencoder("iso2022_jp_3")()
enc3.encode("丨")
print("enc state plane1:", enc3.getstate())

# Incremental decoder over three-byte chunks reassembles the whole string.
dec = codecs.getincrementaldecoder("iso2022_jp_3")()
out = ""
step = 3
for i in range(0, len(data), step):
    out += dec.decode(data[i:i + step], False)
out += dec.decode(b"", True)
print("incremental decode eq:", out == text)

# The decoder carries the four-byte designation across a chunk boundary.
dec2 = codecs.getincrementaldecoder("iso2022_jp_3")()
print("buffered partial:", repr(dec2.decode(b"a\x1b$(", False)))
print("decoder state:", dec2.getstate())
print("completed plane1:", dec2.decode(b"O\x2e\x24", True))

# Stream writer and reader roundtrip.
buf = io.BytesIO()
writer = codecs.getwriter("iso2022_jp_3")(buf)
writer.write(text)
writer.flush()
buf.seek(0)
reader = codecs.getreader("iso2022_jp_3")(buf)
print("stream read eq:", reader.read() == text)

# The yen and overline specials the base codec accepts are unencodable here.
try:
    "¥".encode("iso2022_jp_3")
except UnicodeEncodeError as e:
    print("yen encode strict:", e)
try:
    "‾".encode("iso2022_jp_3")
except UnicodeEncodeError as e:
    print("overline encode strict:", e)

# Error handling on decode: a plane 1 pair with a bad trail is illegal two bytes
# wide; strict raises, ignore drops it, replace emits U+FFFD.
bad = b"\x1b$(O\x21\x20\x1b(B"
try:
    bad.decode("iso2022_jp_3")
except UnicodeDecodeError as e:
    print("decode strict:", e)
print("decode ignore:", repr(bad.decode("iso2022_jp_3", "ignore")))
print("decode replace:", repr(bad.decode("iso2022_jp_3", "replace")))

# Encode error handling: an unrepresentable code point raises under strict and emits
# '?' under replace, returning to ascii first.
try:
    "\U0001F600".encode("iso2022_jp_3")
except UnicodeEncodeError as e:
    print("encode strict:", e)
print("encode replace hex:", ("丨" + "\U0001F600").encode("iso2022_jp_3", "replace").hex())
