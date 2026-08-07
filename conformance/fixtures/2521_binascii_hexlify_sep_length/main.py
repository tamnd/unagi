import binascii


def show(label, fn):
    try:
        print(label, "=>", repr(fn()))
    except Exception as e:
        print(label, "=>", type(e).__name__, ":", e)


# binascii.hexlify and its b2a_hex alias take an optional single-character
# separator: a longer, empty or str separator raises ValueError "sep must be
# length 1." with a trailing period, the wording bytes.hex already spelled and
# CPython uses for both spellings.
print("== a separator that is not one character raises with a period ==")
show("hexlify sep b'::'", lambda: binascii.hexlify(b"ab", b"::"))
show("hexlify sep b''", lambda: binascii.hexlify(b"ab", b""))
show("hexlify sep '::'", lambda: binascii.hexlify(b"ab", "::"))
show("b2a_hex sep b'::'", lambda: binascii.b2a_hex(b"ab", b"::"))
show("bytes.hex sep '::'", lambda: b"ab".hex("::"))
show("bytearray.hex sep '::'", lambda: bytearray(b"ab").hex("::"))

print("== a single-character separator is accepted ==")
show("hexlify sep b':'", lambda: binascii.hexlify(b"abcd", b":"))
show("hexlify sep b':' bytes 2", lambda: binascii.hexlify(b"abcd", b":", 2))
show("b2a_hex sep 'x'", lambda: binascii.b2a_hex(b"ab", "x"))
show("hexlify sep b'\\xff'", lambda: binascii.hexlify(b"ab", b"\xff"))
show("bytes.hex sep ':' bytes 2", lambda: b"abcd".hex(":", 2))
