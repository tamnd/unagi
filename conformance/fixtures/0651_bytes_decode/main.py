# bytes and bytearray decode across the utf-8, ascii and latin-1 codecs.

# Clean decodes.
print(b"abc".decode())
print(b"caf\xc3\xa9".decode("utf-8"))
print(b"caf\xe9".decode("latin-1"))
print(b"abc".decode("ascii"))
print(b"\xf0\x9f\x98\x80".decode())
print(b"a\xe2\x82\xacb".decode("utf-8"))
print(bytearray(b"hi").decode())

# Error handlers.
print(b"a\xffb".decode("utf-8", "ignore"))
print(b"a\xffb".decode("utf-8", "replace"))
print(b"a\xe2\x82b".decode("utf-8", "replace"))
print(b"a\x80b".decode("ascii", "ignore"))
print(b"a\x80b".decode("ascii", "replace"))

# Strict error catalog.
try:
    b"a\xffb".decode("utf-8")
except UnicodeDecodeError as e:
    print("startbyte", e)
try:
    b"a\xc3".decode("utf-8")
except UnicodeDecodeError as e:
    print("shortend", e)
try:
    b"a\xc3\x28".decode("utf-8")
except UnicodeDecodeError as e:
    print("badcont", e)
try:
    b"\xe2\x82".decode("utf-8")
except UnicodeDecodeError as e:
    print("rangeend", e)
try:
    b"\xe2\x82\x28".decode("utf-8")
except UnicodeDecodeError as e:
    print("rangecont", e)
try:
    b"\xed\xa0\x80".decode("utf-8")
except UnicodeDecodeError as e:
    print("surrogate", e)
try:
    b"a\x80b".decode("ascii")
except UnicodeDecodeError as e:
    print("asciibad", e)
try:
    b"abc".decode("bogus")
except LookupError as e:
    print("unknownenc", e)
try:
    b"x".decode(5)
except TypeError as e:
    print("enctype", e)
try:
    b"x".decode("utf-8", 5)
except TypeError as e:
    print("errtype", e)
try:
    b"a\xffb".decode("utf-8", "bogus")
except LookupError as e:
    print("badhandler", e)
