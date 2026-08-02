# euc_kr carries the 2350 Wansung Hangul syllables in its table, but the standard
# also defines a KS X 1001:1998 Annex 3 make-up sequence for the modern syllables
# outside that set: eight bytes, 0xa4 0xd4 then three (0xa4, jamo) pairs for the
# leading consonant, vowel and trailing consonant. CPython encodes and decodes
# these so euc_kr covers all 11172 modern syllables, and this pins unagi to the
# same bytes, boundaries and error behaviour.
import codecs

# A syllable outside the Wansung set round-trips through the eight-byte form.
for ch in ["쓔", "가", "힣", "뷁"]:
    data = ch.encode("euc_kr")
    print(ch, data, len(data), data.decode("euc_kr") == ch)

# A string mixing Wansung, make-up and ascii syllables round-trips whole.
text = "한글A쓔글B"
enc = text.encode("euc_kr")
print("mixed", enc, enc.decode("euc_kr") == text)

# A truncated make-up sequence is incomplete over the bytes in hand.
try:
    b"\xa4\xd4\xa4\xb6".decode("euc_kr")
except UnicodeDecodeError as exc:
    print("truncated", exc)

# A malformed make-up sequence is one illegal byte, so replace resyncs on.
print("malformed", b"\xa4\xd4\x41\xb6\xa4\xd0\xa4\xd4".decode("euc_kr", "replace"))

# The incremental decoder holds a make-up sequence split across two feeds.
dec = codecs.getincrementaldecoder("euc_kr")()
first = "쓔".encode("euc_kr")[:3]
print("inc first", repr(dec.decode(first)))
print("inc rest", dec.decode("쓔".encode("euc_kr")[3:]))
