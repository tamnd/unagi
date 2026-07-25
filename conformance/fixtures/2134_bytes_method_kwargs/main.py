# The bytes and bytearray methods that accept keyword arguments on CPython 3.14.
print(b"a b c".split(sep=b" "))
print(b"a b c".split(b" ", maxsplit=1))
print(b"a b c".split(maxsplit=1))
print(b"a-b-c".rsplit(sep=b"-", maxsplit=1))
print(b"x\ny\n".splitlines(keepends=True))
print(b"data".decode(encoding="ascii", errors="strict"))
print(b"da\xffta".decode(errors="replace"))
print(b"a\tb".expandtabs(tabsize=4))
print(b"abc".translate(None, delete=b"b"))

# bytearray takes the same keywords and keeps its own type.
print(bytearray(b"a b").split(sep=b" "))
print(bytearray(b"abc").translate(None, delete=b"b"))

# base64.b16decode reaches translate(None, delete=...) at import.
import base64
print(base64.b16decode(b"48656C6C6F"))

# The arg-clinic errors match CPython.
def err(f):
    try:
        f()
    except TypeError as e:
        print(e)

err(lambda: b"a".split(bad=1))
err(lambda: b"a b".split(b" ", sep=b" "))
err(lambda: b"ab".decode("utf-8", encoding="utf-8"))
err(lambda: b"a\nb".splitlines(True, keepends=True))
err(lambda: b"a\tb".expandtabs(4, tabsize=8))
err(lambda: b"abc".translate(None, bad=b"b"))
err(lambda: b"abc".translate(None, b"x", delete=b"b"))
err(lambda: b"aaa".replace(b"a", b"b", count=1))
