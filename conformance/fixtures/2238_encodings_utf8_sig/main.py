# utf-8-sig is a pure-Python codec in the encodings package that wraps the
# utf-8 accelerator with BOM handling: it prepends the UTF-8 BOM on encode and
# strips a leading one on decode. The encodings search function resolves it by
# importing encodings.utf_8_sig from a runtime string, so it only works when
# that submodule is compiled into the program; this drives the whole path.
import codecs
import encodings.utf_8_sig as sig

# The stateless codec prepends the BOM on encode and drops it on decode.
print(codecs.encode("caf\xe9-世", "utf-8-sig"))
print(codecs.decode(b"\xef\xbb\xbfcaf\xc3\xa9", "utf-8-sig"))

# A decode without a leading BOM passes the bytes straight through.
print(codecs.decode(b"caf\xc3\xa9", "utf-8-sig"))

# lookup folds the name to the registered codec and its CodecInfo round-trips.
info = codecs.lookup("utf-8-sig")
print(type(info).__name__, info.name)
print(info.encode("a\xe9"), info.decode(b"\xef\xbb\xbfa\xc3\xa9"))

# str.encode and bytes.decode reach the same codec.
print("hi".encode("utf-8-sig"), b"\xef\xbb\xbfhi".decode("utf-8-sig"))

# The incremental encoder writes the BOM only once, before the first chunk.
enc = codecs.getincrementalencoder("utf-8-sig")()
print(enc.encode("a"), enc.encode("b"), enc.encode("c"))

# The incremental decoder strips a BOM split across the first two feeds.
dec = codecs.getincrementaldecoder("utf-8-sig")()
print(repr(dec.decode(b"\xef\xbb")), repr(dec.decode(b"\xbfhi")))

# The submodule imported by name is the codec module itself.
print(sig.__name__)
