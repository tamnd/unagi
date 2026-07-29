# PEP 383 surrogateescape and surrogatepass now work end to end. A str can hold
# a lone surrogate (stored internally as WTF-8), so undecodable filesystem bytes
# round-trip through decode/encode unchanged, and len/index/slice/iter/repr all
# treat the surrogate as one code point. This is what the harness helper modules
# reach when they os.fsdecode non-ASCII path names.
import os

# surrogateescape: an undecodable byte escapes to a low surrogate and back.
raw = b"a\xff\x81b"
s = raw.decode("utf-8", "surrogateescape")
print("escape len", len(s))
print("escape ord", ord(s[1]), ord(s[2]))
print("escape round", s.encode("utf-8", "surrogateescape") == raw)

# The narrow ascii codec escapes every high byte too.
a = b"\x80\xfeZ".decode("ascii", "surrogateescape")
print("ascii round", a.encode("ascii", "surrogateescape") == b"\x80\xfeZ")

# surrogatepass: a real surrogate code point survives utf-8 both ways.
p = b"\xed\xb2\x80".decode("utf-8", "surrogatepass")
print("pass ord", ord(p))
print("pass round", p.encode("utf-8", "surrogatepass") == b"\xed\xb2\x80")

# strict encoding of a lone surrogate is an error.
try:
    s.encode("utf-8", "strict")
    print("strict", "no error")
except UnicodeEncodeError:
    print("strict", "UnicodeEncodeError")

# os.fsdecode / os.fsencode use surrogateescape on posix and round-trip.
print("fsdecode round", os.fsencode(os.fsdecode(raw)) == raw)

# Indexing, slicing and iteration count the surrogate as one element.
print("index", ord(s[1]))
print("slice len", len(s[1:3]))
print("iter len", len(list(s)))

# repr escapes a lone surrogate with a \\udcXX sequence.
print("repr", repr(s))
