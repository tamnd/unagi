import binascii


def show(label, fn):
    try:
        print(label, "=>", repr(fn()))
    except Exception as e:
        print(label, "=>", type(e).__name__, ":", e)


# The b2a_* encoders and the crc functions take a bytes-like object, so a str is
# the bytes-like TypeError rather than a silent UTF-8 encode.
for name in ["b2a_base64", "b2a_hex", "b2a_qp", "b2a_uu", "hexlify", "crc32"]:
    show("b2a_" + name, (lambda n: (lambda: getattr(binascii, n)("test")))(name))
show("crc_hqx_str", lambda: binascii.crc_hqx("test", 0))

# The a2b_* decoders accept a pure-ASCII str and decode it the same as bytes.
show("a2b_hex_ascii", lambda: binascii.a2b_hex("4142"))
show("a2b_qp_ascii", lambda: binascii.a2b_qp("caf=E9"))
show("a2b_base64_ascii", lambda: binascii.a2b_base64("dGVzdA=="))

# A non-ASCII str is the ValueError the ascii_buffer converter raises, ahead of
# any codec-specific decode error.
for name in ["a2b_base64", "a2b_hex", "a2b_qp", "a2b_uu", "unhexlify"]:
    show("nonascii_" + name, (lambda n: (lambda: getattr(binascii, n)("\x80")))(name))
