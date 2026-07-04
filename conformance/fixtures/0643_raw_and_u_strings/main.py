# A raw string keeps every backslash literally, so no escape is decoded and
# no invalid-escape warning fires.
print(repr(r"\n\t\q"))
print(repr(r"C:\path\to\file"))
print(repr(r"back\\slash"))

# The backslash before a quote is kept along with the quote, and the quote
# does not end the string.
print(repr(r"a\"b"), len(r"a\"b"))

# The prefix is case insensitive, and triple-quoted raw strings work too.
print(repr(R"\x41"))
print(repr(r"""line\none"""))

# A legacy u-prefix string is exactly a plain str, so its escapes decode.
print(repr(u"caf\xe9"))
print(repr(u"plain"), repr(U"€"))
