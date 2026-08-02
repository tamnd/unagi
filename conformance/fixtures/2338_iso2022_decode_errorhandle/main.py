# The iso-2022 decoders follow CPython's iso2022processesc: after an ISO-2022
# header byte the decoder scans up to a final byte (A-Z or @), so an unterminated
# escape is incomplete over the whole span in hand and folds to one U+FFFD under
# replace, while a terminated but unrecognized designation is illegal over its own
# span only. SS2 (ESC N) over the ground ascii G2 passes the byte through on the
# codecs that carry a G2 set and is a plain ESC passthrough on the ones that do
# not. euc_jp is the non-strict JIS-Roman variant, so the yen sign and the overline
# fold onto the ascii bytes 0x5c and 0x7e on encode. This pins unagi to CPython.
import codecs


def dec(enc, data, errors):
    return codecs.getdecoder(enc)(data, errors)[0]


# An unterminated escape after a header byte is incomplete over the whole span, so
# replace folds ESC $ d e f to a single U+FFFD on both a G0 designator (iso2022_jp)
# and a G1 designator (iso2022_kr).
print("jp trunc", repr(dec("iso2022_jp", b"ab\x1b$def", "replace")))
print("kr trunc", repr(dec("iso2022_kr", b"ab\x1b$def", "replace")))

# SS2 (ESC N) over the ground ascii G2 passes the byte through on iso2022_jp_2 but
# is a plain ESC passthrough on iso2022_jp, which carries no G2 set.
print("jp2 ss2", repr(dec("iso2022_jp_2", b"ab\x1bNdef", "replace")))
print("jp ss2", repr(dec("iso2022_jp", b"ab\x1bNdef", "replace")))

# A recognized designation followed by an invalid pair is illegal over just the
# pair, so only that pair folds to U+FFFD (iso2022_jp_3 carries JIS X 0213).
print("jp3 pair", repr(dec("iso2022_jp_3", b"ab\x1b$(O\x2e\x21\x1b(Bdef", "replace")))

# An unrecognized but terminated designation is illegal over its own span only.
print("jp bad desig", repr(dec("iso2022_jp", b"ab\x1b(Zdef", "replace")))

# euc_jp folds the yen sign and the overline onto ascii 0x5c and 0x7e on encode.
enc = codecs.getencoder("euc_jp")
print("yen", enc("\xa5")[0])
print("overline", enc("‾")[0])
