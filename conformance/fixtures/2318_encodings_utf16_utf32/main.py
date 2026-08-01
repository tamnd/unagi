import codecs

# Endian-specific encode writes fixed byte order with no BOM.
print("le", "A".encode("utf-16-le"))
print("be", "A".encode("utf-16-be"))
print("32le", "A".encode("utf-32-le"))
print("32be", "A".encode("utf-32-be"))

# The plain codecs prepend the native BOM on encode.
print("plain16", "A".encode("utf-16"))
print("plain32", "A".encode("utf-32"))

# An astral character becomes a surrogate pair in utf-16 and one unit in utf-32.
print("emoji16", "\U0001F600".encode("utf-16-le"))
print("emoji32", "\U0001F600".encode("utf-32-le"))

# Round-trip a mix of ascii, latin and astral through every variant.
sample = "hi é中\U0001F600!"
for enc in ("utf-16", "utf-16-le", "utf-16-be", "utf-32", "utf-32-le", "utf-32-be"):
    print("roundtrip", enc, sample.encode(enc).decode(enc) == sample)

# Plain decode strips a leading BOM and picks its byte order.
print("declebom", codecs.utf_16_decode(b"\xff\xfeA\x00"))
print("decbebom", codecs.utf_16_decode(b"\xfe\xff\x00A"))
print("decnobom", codecs.utf_16_decode(b"A\x00"))
print("dec32lebom", codecs.utf_32_decode(b"\xff\xfe\x00\x00A\x00\x00\x00"))
print("dec32bebom", codecs.utf_32_decode(b"\x00\x00\xfe\xff\x00\x00\x00A"))

# ex_decode reports the byte order it resolved.
print("exnobom", codecs.utf_16_ex_decode(b"A\x00", "strict", 0, True))
print("exlebom", codecs.utf_16_ex_decode(b"\xff\xfeA\x00", "strict", 0, True))
print("exbebom", codecs.utf_16_ex_decode(b"\xfe\xff\x00A", "strict", 0, True))
print("ex32nobom", codecs.utf_32_ex_decode(b"A\x00\x00\x00", "strict", 0, True))

# The incremental decoder, driven from the encodings module, streams a BOM'd input
# split across feeds.
dec = codecs.getincrementaldecoder("utf-16")()
data = "stream".encode("utf-16")
print("inc1", dec.decode(data[:5], False))
print("inc2", dec.decode(data[5:], True))

# The incremental encoder writes the BOM once, then bare units.
enc = codecs.getincrementalencoder("utf-16")()
print("incenc1", enc.encode("ab"))
print("incenc2", enc.encode("cd"))

# Decode error handling on raw bytes.
def guard(label, fn):
    try:
        print(label, fn())
    except (UnicodeDecodeError, UnicodeEncodeError) as e:
        print(label, "ERR", e)

guard("oddfinal", lambda: codecs.utf_16_le_decode(b"A\x00B", "strict", True))
print("oddnonfinal", codecs.utf_16_le_decode(b"A\x00B", "strict", False))
guard("badlow", lambda: codecs.utf_16_le_decode(b"\x00\xd8\x00\x00", "strict", True))
guard("lonelow", lambda: codecs.utf_16_le_decode(b"\x00\xdc", "strict", True))
guard("u32surr", lambda: codecs.utf_32_le_decode(b"\x00\xd8\x00\x00", "strict", True))
guard("u32big", lambda: codecs.utf_32_le_decode(b"\x00\x00\x11\x00", "strict", True))
guard("u32trunc", lambda: codecs.utf_32_le_decode(b"A\x00\x00", "strict", True))
print("lonehighreplace", codecs.utf_16_le_decode(b"\x00\xd8", "replace", True))
print("lonehighignore", codecs.utf_16_le_decode(b"\x00\xd8", "ignore", True))
print("lonehighhold", codecs.utf_16_le_decode(b"\x00\xd8", "strict", False))
