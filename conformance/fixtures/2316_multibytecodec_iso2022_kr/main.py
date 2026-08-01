# iso2022_kr on the _multibytecodec engine, driven through the vendored
# encodings/iso2022_kr.py. iso2022_kr is the Korean member of the ISO-2022 family
# and does not use the G0 escape-designation model the JP codecs do: it designates
# KSC 5601 into G1 once with ESC$)C and then shifts between ascii and KSC 5601 with
# the SO (0x0e) and SI (0x0f) control bytes. Bytes after SO are KSC 5601 GL pairs,
# bytes after SI are ascii. A newline (0x0a, and only 0x0a) resets the shift back to
# ascii on decode while the G1 designation persists. Every non-ascii character routes
# through KSC 5601. getstate packs the G1 designation and the shift state, and all of
# it must match CPython byte for byte.
import codecs
import io

text = "AB 한국어 x"

data = text.encode("iso2022_kr")
print("encoded hex:", data.hex())
print("decoded eq:", data.decode("iso2022_kr") == text)
print("codecs.encode eq:", codecs.encode(text, "iso2022_kr") == data)
print("codecs.decode eq:", codecs.decode(data, "iso2022_kr") == text)

# The designation ESC$)C is emitted once, right before the first SO, and the shift
# returns to ascii with SI before any ascii byte and at the end.
print("single hex:", "한".encode("iso2022_kr").hex())
print("mixed hex:", "한A한".encode("iso2022_kr").hex())
print("ascii only hex:", "AB".encode("iso2022_kr").hex())

# A newline in the KSC shift resets to ascii, so the bytes after it decode as ascii.
print("newline reset:", repr(b"\x1b$)C\x0eE0\x0aE0\x0f".decode("iso2022_kr")))

info = codecs.lookup("iso2022_kr")
print("info name:", info.name)
print("info encode eq:", info.encode(text)[0] == data)
print("info decode eq:", info.decode(data)[0] == text)

# Incremental encoder over one character at a time yields the same bytes; the SI that
# closes the KSC run is deferred to the final call.
enc = codecs.getincrementalencoder("iso2022_kr")()
chunks = b""
for ch in text:
    chunks += enc.encode(ch)
chunks += enc.encode("", True)
print("incremental encode eq:", chunks == data)

# getstate packs the G1 designation and the open shift after a KSC character.
enc2 = codecs.getincrementalencoder("iso2022_kr")()
enc2.encode("한")
print("enc state ksc:", enc2.getstate())
enc3 = codecs.getincrementalencoder("iso2022_kr")()
enc3.encode("A")
print("enc state ascii:", enc3.getstate())

# Incremental decoder over two-byte chunks reassembles the whole string.
dec = codecs.getincrementaldecoder("iso2022_kr")()
out = ""
for i in range(0, len(data), 2):
    out += dec.decode(data[i:i + 2], False)
out += dec.decode(b"", True)
print("incremental decode eq:", out == text)

# The decoder carries the G1 designation and the open shift across a chunk boundary,
# and getstate reports them the way CPython packs them.
dec2 = codecs.getincrementaldecoder("iso2022_kr")()
print("designate partial:", repr(dec2.decode(b"\x1b$)C\x0e", False)))
print("dec state shifted:", dec2.getstate())
print("dec completes:", dec2.decode(b"\x47\x51\x0f", True))

# Stream writer and reader roundtrip.
buf = io.BytesIO()
writer = codecs.getwriter("iso2022_kr")(buf)
writer.write(text)
writer.flush()
buf.seek(0)
reader = codecs.getreader("iso2022_kr")(buf)
print("stream read eq:", reader.read() == text)

# Error handling on decode: a KSC 5601 pair with a bad trail is illegal two bytes
# wide; strict raises, ignore drops it, replace emits U+FFFD.
bad = b"\x1b$)C\x0e\x21\x20\x0f"
try:
    bad.decode("iso2022_kr")
except UnicodeDecodeError as e:
    print("decode strict:", e)
print("decode ignore:", repr(bad.decode("iso2022_kr", "ignore")))
print("decode replace:", repr(bad.decode("iso2022_kr", "replace")))

# A high byte in the ascii shift is illegal one byte wide.
try:
    b"\x80".decode("iso2022_kr")
except UnicodeDecodeError as e:
    print("decode high byte:", e)

# Encode error handling: an unrepresentable code point raises under strict and emits
# '?' under replace, returning to the ascii shift first.
try:
    "\U0001F600".encode("iso2022_kr")
except UnicodeEncodeError as e:
    print("encode strict:", e)
print("encode replace hex:", ("한" + "\U0001F600").encode("iso2022_kr", "replace").hex())
