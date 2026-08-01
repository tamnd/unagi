import codecs

# The utf-7 codec resolves through the encodings package and its aliases.
print("name", codecs.lookup("utf-7").name)
for alias in ["u7", "utf7", "unicode-1-1-utf-7"]:
    print("alias", alias, codecs.lookup(alias).name)

# Encode: a direct run stays literal, '+' becomes '+-', and everything else
# shifts into a base64 section closed by '-' or an implicit direct character.
samples = [
    "",
    "Hello, World!",
    "1 + 1 = 2",
    "a\tb\nc\rd",
    "cost: 100€",          # euro sign shifts in
    "Hi Mom -☺-!",         # smiley between direct dashes
    "\\ and ~ shift too",       # backslash and tilde are not direct
    "ééé",       # a run of shifted characters
    "mix 中文 text",    # CJK
    "emoji \U0001f600 end",     # astral, encoded as a surrogate pair
    "++--",                     # plus and dash edges
]
for s in samples:
    b = s.encode("utf-7")
    print("enc", repr(b), b.decode("utf-7") == s)

# A lone surrogate built at runtime shifts in as its raw 16-bit value.
lone = chr(0xD800) + "x"
print("lone", repr(lone.encode("utf-7")), lone.encode("utf-7").decode("utf-7") == lone)

# Decode: literal bytes, the '+-' escape, and shifted sections including a
# trailing section closed at end of input.
for data in [b"", b"plain text", b"+-", b"a+ACY-b", b"+Jjo-", b"+AGkAaQ", b"+AGk-x", b"1 +- 1"]:
    print("dec", repr(data), repr(data.decode("utf-7")))

# Decode error wording for each failure mode (str() of the raised error).
for data in [b"+IK.", b"+@", b"+//", b"+AGkA", b"++", b"\x80", b"+AAAA-"]:
    try:
        data.decode("utf-7")
        print("dec-ok", repr(data))
    except UnicodeDecodeError as e:
        print("dec-err", repr(data), str(e))

# The ignore and replace handlers recover from a bad shift.
for data in [b"+IK.rest", b"\x80abc", b"good+@bad"]:
    print("ign", repr(data), repr(data.decode("utf-7", "ignore")))
    print("rep", repr(data), repr(data.decode("utf-7", "replace")))

# The incremental decoder splits at every byte and rebuilds the whole string.
dec = codecs.getincrementaldecoder("utf-7")()
b = "a ☺ b \U0001f600 c".encode("utf-7")
parts = [dec.decode(b[k:k + 1], False) for k in range(len(b))]
parts.append(dec.decode(b"", True))
print("incremental", "".join(parts) == b.decode("utf-7"))
