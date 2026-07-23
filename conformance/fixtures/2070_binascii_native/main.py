# binascii is the C accelerator base64 converts binary to ASCII on, imported
# directly with no pure fallback, so `import base64` needs it. This slice
# carries the base64 and hex codecs base64 drives plus the two CRC helpers and
# the Error exception; the values here are pure ASCII, identical on every host.

import binascii

_names = ["Error", "Incomplete", "a2b_base64", "a2b_hex", "b2a_base64",
          "b2a_hex", "crc32", "crc_hqx", "hexlify", "unhexlify"]
print("attrs", [n for n in _names if hasattr(binascii, n)])
print("Error is ValueError", issubclass(binascii.Error, ValueError))
print("Incomplete", issubclass(binascii.Incomplete, Exception))

# hex round-trips both ways, with an optional separator.
print("hexlify", binascii.hexlify(b"abc"), binascii.b2a_hex(b"\x00\xff"))
print("hexlify sep", binascii.hexlify(b"abcd", b"-", 2))
print("unhexlify", binascii.unhexlify(b"616263"), binascii.a2b_hex("00ff"))

# base64 round-trips, with and without the trailing newline.
print("b2a", binascii.b2a_base64(b"hello"), binascii.b2a_base64(b"hello", newline=False))
print("a2b", binascii.a2b_base64(b"aGVsbG8=\n"))
print("a2b strict", binascii.a2b_base64(b"aGVsbG8=", strict_mode=True))
print("a2b junk", binascii.a2b_base64(b"aG!!Vsb#G8="))

# CRCs match the standard polynomials.
print("crc32", binascii.crc32(b"hello"), binascii.crc32(b"lo", binascii.crc32(b"hel")))
print("crc_hqx", binascii.crc_hqx(b"hello", 0))


def err(fn):
    try:
        fn()
        print("NOERR")
    except binascii.Error as e:
        print("Error", e)


err(lambda: binascii.unhexlify(b"abc"))
err(lambda: binascii.unhexlify(b"zz"))
err(lambda: binascii.a2b_base64(b"aGVsbG8"))
err(lambda: binascii.a2b_base64(b"A===", strict_mode=True))
err(lambda: binascii.a2b_base64(b"====", strict_mode=True))
err(lambda: binascii.a2b_base64(b"aG!V", strict_mode=True))
err(lambda: binascii.a2b_base64(b"aGVsbG8=extra", strict_mode=True))

# base64 drives exactly this surface (b2a_base64/a2b_base64/hexlify), so the
# module is ready for it; base64 itself additionally needs bytes.maketrans, a
# separate builtin-type static-method gap tracked apart from binascii.
