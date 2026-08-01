# hz on the _multibytecodec engine, driven through the vendored encodings/hz.py.
# hz is the shift-state member of the Chinese codec family: the byte stream toggles
# between an ascii mode and a GB mode with the escape pairs ~{ (enter GB) and ~}
# (return to ascii), ~~ stands for a literal tilde, and in GB mode a byte pair maps
# a gb2312 character with the high bit stripped. The mode is carried across bytes
# and across chunk boundaries, so this is the first codec on the engine's
# shift-state driver rather than the per-unit step functions. The text below mixes
# ascii runs, gb2312 characters and a literal tilde so the encoder opens and closes
# GB mode several times. This exercises the stateless encode/decode, the
# incremental encoder and decoder (including a GB pair split across a chunk
# boundary), getstate/setstate, the stream reader and writer, and the
# strict/ignore/replace error handling, all of which must match CPython byte for
# byte.
import codecs
import io

text = "汉字 abc 你好世界 ~tilde~ 十 符号"

# Stateless roundtrip through str.encode / bytes.decode and codecs.encode/decode.
data = text.encode("hz")
print("encoded hex:", data.hex())
print("decoded eq:", data.decode("hz") == text)
print("codecs.encode eq:", codecs.encode(text, "hz") == data)
print("codecs.decode eq:", codecs.decode(data, "hz") == text)

# A literal tilde doubles, and the escape pairs frame each GB run.
print("tilde hex:", "a~b".encode("hz").hex())
print("open close:", "十".encode("hz").hex())

# codecs.lookup exposes the CodecInfo the encodings module builds.
info = codecs.lookup("hz")
print("info name:", info.name)
print("info encode eq:", info.encode(text)[0] == data)
print("info decode eq:", info.decode(data)[0] == text)

# Incremental encoder: feeding one character at a time yields the same bytes. The
# encoder holds the GB mode between calls and flushes ~} on the final empty call.
enc = codecs.getincrementalencoder("hz")()
chunks = b""
for ch in text:
    chunks += enc.encode(ch)
chunks += enc.encode("", True)
print("incremental encode eq:", chunks == data)

# The encoder getstate reports the shift mode: 0 in ascii mode, 256 in GB mode.
enc2 = codecs.getincrementalencoder("hz")()
print("enc state ascii:", enc2.getstate())
enc2.encode("十")
print("enc state gb:", enc2.getstate())

# Incremental decoder over arbitrary byte chunks, including a split that lands in
# the middle of a GB pair, must reassemble the whole string.
dec = codecs.getincrementaldecoder("hz")()
out = ""
step = 3
for i in range(0, len(data), step):
    out += dec.decode(data[i:i + step], False)
out += dec.decode(b"", True)
print("incremental decode eq:", out == text)

# The decoder carries the GB mode across a chunk boundary: enter GB and feed a lone
# lead byte, then complete the pair in the next chunk.
dec2 = codecs.getincrementaldecoder("hz")()
print("buffered partial:", repr(dec2.decode(b"~{o", False)))
print("decoder state:", dec2.getstate())
print("completed pair:", dec2.decode(b";", True))

# getstate/setstate roundtrip mid GB mode.
dec3 = codecs.getincrementaldecoder("hz")()
dec3.decode(b"~{", False)
state = dec3.getstate()
dec4 = codecs.getincrementaldecoder("hz")()
dec4.setstate(state)
print("setstate resumes gb:", dec4.decode(b"o;", True))

# Stream writer and reader roundtrip over an in-memory byte stream: whatever the
# writer emits, the reader decodes it back to the original text.
buf = io.BytesIO()
writer = codecs.getwriter("hz")(buf)
writer.write(text)
writer.flush()
buf.seek(0)
reader = codecs.getreader("hz")(buf)
print("stream read eq:", reader.read() == text)

# Error handling on decode: strict raises, ignore drops the bad span, replace emits
# U+FFFD. An unknown ascii-mode escape is illegal at the tilde.
try:
    b"~x".decode("hz")
except UnicodeDecodeError as e:
    print("decode strict:", e)
print("decode ignore:", repr(b"a~xb".decode("hz", "ignore")))
print("decode replace:", repr(b"a~xb".decode("hz", "replace")))

# In GB mode only ~} is a valid escape, and a bad GB pair is illegal one byte wide
# at the lead.
try:
    b"~{~x".decode("hz")
except UnicodeDecodeError as e:
    print("decode gb escape:", e)
try:
    b"~{o ".decode("hz")
except UnicodeDecodeError as e:
    print("decode bad pair:", e)

# Encode error handling: a code point gb2312 cannot represent raises under strict,
# and replace emits '?' in ascii mode, closing any open GB run first.
try:
    "\U0001F600".encode("hz")
except UnicodeEncodeError as e:
    print("encode strict:", e)
print("encode replace hex:", ("十" + "\U0001F600").encode("hz", "replace").hex())
