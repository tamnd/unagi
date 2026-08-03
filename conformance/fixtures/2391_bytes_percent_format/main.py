def show(label, fn):
    try:
        print(label, "->", repr(fn()))
    except Exception as e:
        print(label, "->", type(e).__name__, e)


class HasBytes:
    def __bytes__(self):
        return b"custom"


# numeric conversions carry over from str formatting
show("d", lambda: b"%d apples" % 5)
show("i pad neg", lambda: b"%05i" % -42)
show("u", lambda: b"%u" % 12)
show("x alt", lambda: b"%#x" % 255)
show("X", lambda: b"%X" % 3735928559)
show("o alt", lambda: b"%#o" % 8)
show("plus", lambda: b"%+d" % 7)
show("space", lambda: b"% d" % 7)
show("f", lambda: b"%.2f" % 3.14159)
show("e", lambda: b"%e" % 12345.678)
show("E", lambda: b"%E" % 12345.678)
show("g small", lambda: b"%g" % 0.0001)
show("bignum d", lambda: b"%d" % (2 ** 70))
show("float d truncates", lambda: b"%d" % 3.9)

# %s and %b want a bytes-like object or __bytes__
show("s bytes", lambda: b"%s!" % b"hi")
show("b bytes", lambda: b"%b!" % b"hi")
show("s bytearray", lambda: b"%s" % bytearray(b"ba"))
show("s memoryview", lambda: b"%s" % memoryview(b"mv"))
show("s dunder", lambda: b"%s" % HasBytes())
show("b dunder", lambda: b"%b" % HasBytes())
show("s prec", lambda: b"%.3s" % b"abcdef")
show("s width", lambda: b"%6s|" % b"ab")
show("s left", lambda: b"%-6s|" % b"ab")
show("s str fails", lambda: b"%s" % "abc")
show("b int fails", lambda: b"%b" % 123)

# %a and %r render the ascii repr as bytes
show("a str", lambda: b"%a" % "café")
show("r bytes", lambda: b"%r" % b"x\ny")
show("a int", lambda: b"%a" % 5)

# %c takes an int in range(256) or a single byte
show("c int", lambda: b"%c" % 65)
show("c byte", lambda: b"%c" % b"Z")
show("c bytearray", lambda: b"%c" % bytearray(b"Q"))
show("c bool", lambda: b"%c" % True)
show("c width", lambda: b"%5c|" % 65)
show("c range hi", lambda: b"%c" % 256)
show("c range lo", lambda: b"%c" % -1)
show("c big", lambda: b"%c" % (2 ** 70))
show("c bytes len", lambda: b"%c" % b"AB")
show("c bytearray len", lambda: b"%c" % bytearray(b"AB"))
show("c float", lambda: b"%c" % 1.5)

# star width and precision, tuple and mapping right operands
show("star width", lambda: b"%*d|" % (4, 7))
show("star neg width", lambda: b"%*d|" % (-5, 3))
show("star prec", lambda: b"%.*f" % (3, 3.14159))
show("tuple", lambda: b"%s-%d" % (b"x", 2))
show("mapping", lambda: b"%(k)s=%(v)d" % {b"k": b"key", b"v": 9})
show("mapping missing", lambda: b"%(z)s" % {b"k": b"v"})
show("mapping needs map", lambda: b"%(k)s" % 5)

# literal percent and argument accounting
show("pct literal", lambda: b"a%%b %d" % 1)
show("empty", lambda: b"" % ())
show("too many", lambda: b"%d" % (1, 2))
show("too few", lambda: b"%d %d" % (1,))
show("bad char", lambda: b"%y" % 1)
show("incomplete", lambda: b"%" % 1)

# type of the result follows the left operand
show("bytes type", lambda: type(b"%d" % 5).__name__)
show("bytearray fmt", lambda: bytearray(b"%d apples") % 5)
show("bytearray type", lambda: type(bytearray(b"%d") % 5).__name__)
