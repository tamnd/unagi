# iso2022_jp_2004 on the _multibytecodec engine, driven through the vendored
# encodings/iso2022_jp_2004.py. iso2022_jp_2004 is the 2004 revision of
# iso2022_jp_3: same repertoire (ascii, JIS X 0208 by ESC$B and the two JIS X 0213
# planes) but plane 1 is designated ESC$(Q instead of ESC$(O. Like iso2022_jp_3 it
# does not designate JIS X 0201 roman, the 1978 revision, katakana or JIS X 0212, so
# the yen and overline specials the base codec folds into ESC(J are unencodable.
#
# Plane 1 carries the same 25 combining sequences (a GL pair decodes to a base plus
# U+309A and the two-code-point sequence encodes back to that one pair). The 2004
# revision also routes one more code point through plane 2 than iso2022_jp_3 does:
# U+9B1C is decode-only under iso2022_jp_3 but encodes as an ESC$(P pair here. All of
# it must match CPython byte for byte.
import codecs
import io

# The text mixes ascii, a JIS X 0208 kanji, a plane 1 combining sequence (U+304B
# U+309A), a plane 1 single character and a trailing ascii run.
text = "AB 漢 か゚ 丨 x"

data = text.encode("iso2022_jp_2004")
print("encoded hex:", data.hex())
print("decoded eq:", data.decode("iso2022_jp_2004") == text)
print("codecs.encode eq:", codecs.encode(text, "iso2022_jp_2004") == data)
print("codecs.decode eq:", codecs.decode(data, "iso2022_jp_2004") == text)

# The combining sequence encodes as one plane 1 pair designated ESC$(Q; a JIS X 0208
# kanji still uses ESC$B.
print("combining hex:", "か゚".encode("iso2022_jp_2004").hex())
print("jisx0208 hex:", "漢".encode("iso2022_jp_2004").hex())
print("plane1 single hex:", "丨".encode("iso2022_jp_2004").hex())

# U+9B1C is the one code point the 2004 revision routes through plane 2 (ESC$(P)
# that iso2022_jp_3 leaves decode-only.
print("plane2 extra hex:", "鬜".encode("iso2022_jp_2004").hex())
print("plane2 extra roundtrip:", "鬜".encode("iso2022_jp_2004").decode("iso2022_jp_2004") == "鬜")

# A plane 2 pair decodes through ESC$(P.
print("plane2 decode:", b"\x1b$(P\x21\x21\x1b(B".decode("iso2022_jp_2004"))

info = codecs.lookup("iso2022_jp_2004")
print("info name:", info.name)
print("info encode eq:", info.encode(text)[0] == data)
print("info decode eq:", info.decode(data)[0] == text)

# Incremental encoder over one character at a time yields the same bytes. The base
# of the combining sequence is held until the mark arrives.
enc = codecs.getincrementalencoder("iso2022_jp_2004")()
chunks = b""
for ch in text:
    chunks += enc.encode(ch)
chunks += enc.encode("", True)
print("incremental encode eq:", chunks == data)

# Feeding a combining base alone yields nothing; the following mark emits the pair.
enc2 = codecs.getincrementalencoder("iso2022_jp_2004")()
print("base held:", enc2.encode("か").hex())
print("mark completes:", enc2.encode("゚", True).hex())

# getstate packs the plane 1 designation into the low byte.
enc3 = codecs.getincrementalencoder("iso2022_jp_2004")()
enc3.encode("丨")
print("enc state plane1:", enc3.getstate())

# Incremental decoder over three-byte chunks reassembles the whole string.
dec = codecs.getincrementaldecoder("iso2022_jp_2004")()
out = ""
step = 3
for i in range(0, len(data), step):
    out += dec.decode(data[i:i + step], False)
out += dec.decode(b"", True)
print("incremental decode eq:", out == text)

# The decoder carries the four-byte designation across a chunk boundary.
dec2 = codecs.getincrementaldecoder("iso2022_jp_2004")()
print("buffered partial:", repr(dec2.decode(b"a\x1b$(", False)))
print("decoder state:", dec2.getstate())
print("completed plane1:", dec2.decode(b"Q\x2e\x24", True))

# Stream writer and reader roundtrip.
buf = io.BytesIO()
writer = codecs.getwriter("iso2022_jp_2004")(buf)
writer.write(text)
writer.flush()
buf.seek(0)
reader = codecs.getreader("iso2022_jp_2004")(buf)
print("stream read eq:", reader.read() == text)

# The yen and overline specials the base codec accepts are unencodable here.
try:
    "¥".encode("iso2022_jp_2004")
except UnicodeEncodeError as e:
    print("yen encode strict:", e)
try:
    "‾".encode("iso2022_jp_2004")
except UnicodeEncodeError as e:
    print("overline encode strict:", e)

# Error handling on decode: a plane 1 pair with a bad trail is illegal two bytes
# wide; strict raises, ignore drops it, replace emits U+FFFD.
bad = b"\x1b$(Q\x21\x20\x1b(B"
try:
    bad.decode("iso2022_jp_2004")
except UnicodeDecodeError as e:
    print("decode strict:", e)
print("decode ignore:", repr(bad.decode("iso2022_jp_2004", "ignore")))
print("decode replace:", repr(bad.decode("iso2022_jp_2004", "replace")))

# Encode error handling: an unrepresentable code point raises under strict and emits
# '?' under replace, returning to ascii first.
try:
    "\U0001F600".encode("iso2022_jp_2004")
except UnicodeEncodeError as e:
    print("encode strict:", e)
print("encode replace hex:", ("丨" + "\U0001F600").encode("iso2022_jp_2004", "replace").hex())
