# namereplace is one of the standard codec error handlers codecs preregisters.
# On an encode error it replaces each unencodable character with its \N{NAME}
# form when the character has a Unicode name, else it falls back to the
# backslashreplace escape (\xNN, \uNNNN or \UNNNNNNNN), and it is encode only.
# It reaches the unicodedata name database the way CPython's namereplace does.
# This pins the handler through str.encode, through codecs.encode, and through a
# direct call on a UnicodeEncodeError looked up from the registry, against
# CPython.
import codecs

# str.encode with the named handler over a few encodings. Each unencodable
# character here has a Unicode name, so the output is the \N{NAME} form.
for s, enc in [
    ("aሴb", "ascii"),
    ("café", "ascii"),
    ("a中b", "ascii"),
    ("a\U0001f600b", "ascii"),
    ("aሴ\U0001f600b", "ascii"),
    ("ÿĀā", "latin-1"),
    ("straße", "ascii"),
]:
    print(repr(s), enc, "->", repr(s.encode(enc, "namereplace")))

# A character with no Unicode name falls back to the backslash escape. U+0080 is
# a C1 control with no name, U+FDD0 is a noncharacter with no name, and a lone
# surrogate has no name either (the case the codec suites check). The input is
# labeled by hand rather than repr'd so the check stays on the encoded bytes.
for label, s in [
    ("U+0080 control", "a\x80b"),
    ("all ascii", "ab"),
    ("U+FDD0 noncharacter", "a﷐b"),
    ("lone surrogate", "[\udc80]"),
]:
    print(label, "->", repr(s.encode("ascii", "namereplace")))

# A mix of named and unnamed characters in one run.
print("mix ->", repr("ሴ\x80\U0001f600".encode("ascii", "namereplace")))

# codecs.encode routes through the same registered handler.
print("codecs.encode ->", repr(codecs.encode("aሴb", "ascii", "namereplace")))

# The handler is registered under the name and resolvable through lookup_error.
h = codecs.lookup_error("namereplace")
print("lookup_error ok:", h is not None)

# Called directly on a UnicodeEncodeError the codec loop would hand it, it returns
# the (replacement, newpos) pair.
err = UnicodeEncodeError("ascii", "aሴ\U0001f600b", 1, 3, "ordinal not in range(128)")
print("direct call ->", h(err))

# A single-character span.
err = UnicodeEncodeError("ascii", "aሴb", 1, 2, "ordinal not in range(128)")
print("single span ->", codecs.namereplace_errors(err))

# namereplace is encode only: handed a decode error it raises TypeError.
try:
    codecs.namereplace_errors(UnicodeDecodeError("ascii", b"a\xffb", 1, 2, "ordinal not in range(128)"))
except TypeError as e:
    print("decode TypeError:", e)

# Handed something that is not a unicode error at all it also raises TypeError.
try:
    codecs.namereplace_errors(ValueError("nope"))
except TypeError as e:
    print("non-error TypeError:", e)
