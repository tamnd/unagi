# The _codecs decode accelerators honor the incremental `final` flag. With
# final false a multibyte sequence cut short at the end of the buffer is held
# back rather than reported, so the accelerator returns the decoded prefix and a
# consumed count short of the input; with final true (or a clean buffer) it
# consumes everything and a truncated tail raises. This is what lets the codecs
# module's StreamReader and incremental decoder reassemble UTF-8 that straddles a
# read boundary instead of failing on the split.
import codecs
from io import BytesIO

# A three-byte character split after its first two bytes.
part = "世".encode("utf-8")[:2]
print("two arg", codecs.utf_8_decode(part, "strict"))
print("final false", codecs.utf_8_decode(part, "strict", False))
try:
    codecs.utf_8_decode(part, "strict", True)
except UnicodeDecodeError as exc:
    print("final true", type(exc).__name__, exc)

# An invalid (not merely incomplete) tail still raises even without final.
try:
    codecs.utf_8_decode(b"a\xff", "strict", False)
except UnicodeDecodeError as exc:
    print("bad tail", type(exc).__name__, exc)

# A complete buffer reports the full consumed count.
print("complete", codecs.utf_8_decode("世界".encode("utf-8"), "strict", False))

# The StreamReader reassembles multibyte runs across every chunk size.
text = "こんにちは世界🍣"
data = text.encode("utf-8")
for sizehint in [1, 2, 3, 5, 7]:
    reader = codecs.getreader("utf-8")(BytesIO(data))
    out = []
    while True:
        chunk = reader.read(sizehint)
        if not chunk:
            break
        out.append(chunk)
    print("read", sizehint, "".join(out) == text)

# The incremental decoder holds a split sequence until the next feed.
dec = codecs.getincrementaldecoder("utf-8")()
print("inc first", repr(dec.decode(part)))
print("inc rest", dec.decode("世".encode("utf-8")[2:]))
