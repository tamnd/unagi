def show(label, fn):
    try:
        print(label, repr(fn()))
    except Exception as ex:
        print(label, "ERR", type(ex).__name__, str(ex))


def released():
    m = memoryview(bytearray(b"ab"))
    m.release()
    return m.hex("-")


# memoryview.hex now takes the same optional separator and bytes-per-group
# arguments bytes.hex and bytearray.hex accept, grouping the underlying buffer
# bytes so a plain call is unchanged, a one-character separator joins each byte,
# a positive group size groups from the left and a negative one from the right,
# and a cast view groups its logical bytes one per group rather than per element.
show("plain", lambda: memoryview(b"hello").hex())
show("sep", lambda: memoryview(b"hello").hex("-"))
show("sep-bytes", lambda: memoryview(b"hello").hex(b":"))
show("sep-n", lambda: memoryview(b"abcdef").hex("-", 2))
show("sep-neg", lambda: memoryview(b"abcdef").hex("_", -2))
show("sep-empty", lambda: memoryview(b"").hex("-"))
show("sep-empty-n", lambda: memoryview(b"").hex("-", 2))
show("cast-sep", lambda: memoryview(b"\x01\x02\x03\x04").cast("H").hex("-"))
show("cast-sep-n", lambda: memoryview(b"\x01\x02\x03\x04\x05\x06").cast("H").hex("-", 2))
show("bytearray-backed", lambda: memoryview(bytearray(b"world")).hex("."))

# The separator validation matches bytes.hex too: a multi-character separator is
# the length ValueError, a non-ASCII one the ASCII ValueError, a separator with
# no length the len() TypeError, a non-integer group size the cannot-be-
# interpreted TypeError, too many arguments the arity TypeError and a released
# view the released-memoryview ValueError.
show("multichar", lambda: memoryview(b"ab").hex("--"))
show("nonascii", lambda: memoryview(b"ab").hex("é"))
show("badtype", lambda: memoryview(b"ab").hex(1))
show("bad-bps", lambda: memoryview(b"ab").hex("-", "x"))
show("too-many", lambda: memoryview(b"ab").hex("-", 2, 3))
show("released", released)
