# iso2022_jp on the _multibytecodec engine, driven through the vendored
# encodings/iso2022_jp.py. iso2022_jp is the base of the ISO-2022 family and the
# second shift-state codec on the engine (after hz): ESC sequences designate the G0
# charset among ascii (ESC(B), JIS X 0201 roman (ESC(J) and JIS X 0208 (ESC$B, and
# ESC$@ for the 1978 revision on decode), and the bytes that follow are read in
# that charset until the next designation. The encoder returns to ascii before any
# ascii byte and at the end of the text. The designation is carried across bytes
# and chunk boundaries, and getstate packs it into the codec state the way CPython
# does. The text below mixes ascii, hiragana, kanji, the two roman specials (yen
# and overline) and a newline so the encoder opens and closes several designations.
# This exercises the stateless encode/decode, the incremental encoder and decoder
# (including a designation escape split across a chunk boundary), getstate/setstate,
# the stream roundtrip, and the strict/ignore/replace error handling, all of which
# must match CPython byte for byte.
import codecs
import io

text = "ABC あいう 漢字 ¥‾ ABC\n漢"

# Stateless roundtrip through str.encode / bytes.decode and codecs.encode/decode.
data = text.encode("iso2022_jp")
print("encoded hex:", data.hex())
print("decoded eq:", data.decode("iso2022_jp") == text)
print("codecs.encode eq:", codecs.encode(text, "iso2022_jp") == data)
print("codecs.decode eq:", codecs.decode(data, "iso2022_jp") == text)

# The roman specials designate ESC(J, and the encoder returns to ascii at the end.
print("yen hex:", "¥".encode("iso2022_jp").hex())
print("kanji hex:", "漢".encode("iso2022_jp").hex())

# Both 0208 revisions decode through the same table; ESC$@ is the 1978 revision.
print("jis1978 eq:", (b"\x1b$@4A\x1b(B").decode("iso2022_jp") == "漢")

# codecs.lookup exposes the CodecInfo the encodings module builds.
info = codecs.lookup("iso2022_jp")
print("info name:", info.name)
print("info encode eq:", info.encode(text)[0] == data)
print("info decode eq:", info.decode(data)[0] == text)

# Incremental encoder: feeding one character at a time yields the same bytes. The
# encoder holds the designation between calls and returns to ascii on the final
# empty call.
enc = codecs.getincrementalencoder("iso2022_jp")()
chunks = b""
for ch in text:
    chunks += enc.encode(ch)
chunks += enc.encode("", True)
print("incremental encode eq:", chunks == data)

# The encoder getstate packs the current designation: the ascii ground state and
# the two-byte state differ.
enc2 = codecs.getincrementalencoder("iso2022_jp")()
print("enc state ascii:", enc2.getstate())
enc2.encode("漢")
print("enc state 0208:", enc2.getstate())

# Incremental decoder over arbitrary byte chunks, including a split that lands in
# the middle of a designation escape, must reassemble the whole string.
dec = codecs.getincrementaldecoder("iso2022_jp")()
out = ""
step = 3
for i in range(0, len(data), step):
    out += dec.decode(data[i:i + step], False)
out += dec.decode(b"", True)
print("incremental decode eq:", out == text)

# The decoder carries the designation across a chunk boundary: feed a partial
# escape, then complete it and a kanji pair in the next chunk.
dec2 = codecs.getincrementaldecoder("iso2022_jp")()
print("buffered partial:", repr(dec2.decode(b"a\x1b$", False)))
print("decoder state:", dec2.getstate())
print("completed kanji:", dec2.decode(b"B4A", True))

# getstate/setstate roundtrip mid two-byte mode.
dec3 = codecs.getincrementaldecoder("iso2022_jp")()
dec3.decode(b"\x1b$B", False)
state = dec3.getstate()
dec4 = codecs.getincrementaldecoder("iso2022_jp")()
dec4.setstate(state)
print("setstate resumes 0208:", dec4.decode(b"4A\x1b(B", True))

# Stream writer and reader roundtrip over an in-memory byte stream.
buf = io.BytesIO()
writer = codecs.getwriter("iso2022_jp")(buf)
writer.write(text)
writer.flush()
buf.seek(0)
reader = codecs.getreader("iso2022_jp")(buf)
print("stream read eq:", reader.read() == text)

# Error handling on decode: strict raises, ignore drops the bad span, replace emits
# U+FFFD. A JIS X 0208 pair with a bad trail is illegal two bytes wide.
bad = b"\x1b$B\x21\x20\x1b(B"
try:
    bad.decode("iso2022_jp")
except UnicodeDecodeError as e:
    print("decode strict:", e)
print("decode ignore:", repr(bad.decode("iso2022_jp", "ignore")))
print("decode replace:", repr(bad.decode("iso2022_jp", "replace")))

# A four-byte designation the base codec does not carry is illegal over all four
# bytes, and an ESC not starting a designation is a passthrough control byte.
try:
    b"\x1b$(Da".decode("iso2022_jp")
except UnicodeDecodeError as e:
    print("decode bad designation:", e)
print("esc passthrough:", repr(b"\x1bZ".decode("iso2022_jp")))

# Encode error handling: a code point iso2022_jp cannot represent raises under
# strict, and replace emits '?' in ascii mode, returning from any designation first.
try:
    "\U0001F600".encode("iso2022_jp")
except UnicodeEncodeError as e:
    print("encode strict:", e)
print("encode replace hex:", ("漢" + "\U0001F600").encode("iso2022_jp", "replace").hex())
