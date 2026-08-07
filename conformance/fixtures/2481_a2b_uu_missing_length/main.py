import binascii


def show(label, fn):
    try:
        print(label, "=>", repr(fn()))
    except Exception as e:
        print(label, "=>", type(e).__name__, ":", e)


# An empty input has no length byte to read, so a2b_uu raises binascii.Error
# rather than returning b'', both for a bare empty and a zero-slice.
show("empty", lambda: binascii.a2b_uu(b""))
show("sliced_empty", lambda: binascii.a2b_uu(b"#86)C"[:0]))

# A zero-length line whose length byte is a space or backtick still decodes to
# b'', so the empty guard does not swallow a genuine zero-length line.
show("space_nl", lambda: binascii.a2b_uu(b" \n"))
show("backtick_nl", lambda: binascii.a2b_uu(b"`\n"))

# The documented length-byte examples read back unchanged.
show("len_7f", lambda: binascii.a2b_uu(b"\x7f"))
show("len_80", lambda: binascii.a2b_uu(b"\x80"))
show("len_ff", lambda: binascii.a2b_uu(b"\xff"))

# Over-long and illegal lines still raise the way they did before.
show("ff00", lambda: binascii.a2b_uu(b"\xff\x00"))
show("bang4", lambda: binascii.a2b_uu(b"!!!!"))

# A real round trip through b2a_uu is unaffected.
show("roundtrip", lambda: binascii.a2b_uu(binascii.b2a_uu(b"abc")))
