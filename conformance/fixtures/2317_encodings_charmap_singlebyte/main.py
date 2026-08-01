# The single-byte charmap codecs, driven through the vendored encodings modules and
# the _codecs.charmap_build/encode/decode accelerator behind them. Each of these
# encodings ships a 256-entry decoding table and inverts it with charmap_build; every
# defined byte roundtrips and every undefined byte raises the same "character maps to
# <undefined>" error CPython reports under the codec name "charmap".
import codecs

# One codec from each family: an ISO 8859 part, a Windows code page, an EBCDIC page, a
# DOS page, a KOI8 set, a Mac set, and the national odds and ends.
codecs_list = [
    "iso8859_2", "iso8859_5", "iso8859_7", "iso8859_15",
    "cp1252", "cp1251", "cp1250",
    "cp037", "cp500",
    "cp437", "cp850", "cp866",
    "koi8_r", "koi8_u",
    "mac_roman", "mac_cyrillic", "mac_greek",
    "tis_620", "hp_roman8", "palmos", "ptcp154", "kz1048",
]

for name in codecs_list:
    # Every byte the codec decodes must re-encode to the same byte.
    ok = True
    decoded_count = 0
    for b in range(256):
        try:
            ch = bytes([b]).decode(name)
        except UnicodeDecodeError:
            continue
        decoded_count += 1
        if ch.encode(name) != bytes([b]):
            ok = False
            break
    print(name, "roundtrip", ok, "defined", decoded_count)

# The lookup names resolve and report the canonical codec name.
print("cp1252 info:", codecs.lookup("cp1252").name)
print("iso8859_15 info:", codecs.lookup("iso8859_15").name)

# An undefined byte raises with the charmap wording, and the handlers behave.
try:
    b"\x81".decode("cp1252")
except UnicodeDecodeError as e:
    print("cp1252 undef:", e)
print("cp1252 ignore:", repr(b"A\x81B".decode("cp1252", "ignore")))
print("cp1252 replace:", repr(b"A\x81B".decode("cp1252", "replace")))

# A code point the codec cannot represent raises on encode.
try:
    "€".encode("iso8859_1")
except UnicodeEncodeError as e:
    print("iso8859_1 no euro:", e)
print("iso8859_15 euro hex:", "€".encode("iso8859_15").hex())

# The Windows and Mac sets carry the smart punctuation and currency signs.
print("cp1252 quotes:", "“”".encode("cp1252").hex())
print("mac_roman apple:", "™".encode("mac_roman").hex())

# A known text roundtrips through a Cyrillic page.
ru = "Привет мир"
print("cp1251 ru eq:", ru.encode("cp1251").decode("cp1251") == ru)
print("koi8_r ru eq:", ru.encode("koi8_r").decode("koi8_r") == ru)

# Incremental and stream interfaces work through the charmap machinery.
enc = codecs.getincrementalencoder("cp1252")()
print("cp1252 incremental:", (enc.encode("A") + enc.encode("€") + enc.encode("", True)).hex())
dec = codecs.getincrementaldecoder("cp1251")()
print("cp1251 incremental eq:", (dec.decode(ru.encode("cp1251"), True)) == ru)
