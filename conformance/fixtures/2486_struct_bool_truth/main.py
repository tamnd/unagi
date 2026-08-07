import struct


def show(label, fn):
    try:
        print(label, "=>", repr(fn()))
    except Exception as e:
        print(label, "=>", type(e).__name__, ":", e)


# Plain values take their ordinary truth.
show("true", lambda: struct.pack('?', True).hex())
show("false", lambda: struct.pack('?', False).hex())
show("int0", lambda: struct.pack('?', 0).hex())
show("int7", lambda: struct.pack('?', 7).hex())
show("none", lambda: struct.pack('?', None).hex())
show("emptystr", lambda: struct.pack('?', "").hex())
show("str", lambda: struct.pack('?', "x").hex())


# A __len__ of zero is falsy, non-zero is truthy.
class Sized:
    def __init__(self, n):
        self.n = n

    def __len__(self):
        return self.n


show("len0", lambda: struct.pack('?', Sized(0)).hex())
show("len3", lambda: struct.pack('?', Sized(3)).hex())


# __bool__ wins over __len__ and its exception propagates.
class Boolish:
    def __init__(self, v):
        self.v = v

    def __bool__(self):
        return self.v

    def __len__(self):
        return 0


show("bool_true", lambda: struct.pack('?', Boolish(True)).hex())
show("bool_false", lambda: struct.pack('?', Boolish(False)).hex())


class Exploding:
    def __bool__(self):
        raise ValueError("boom")


show("explode", lambda: struct.pack('?', Exploding()))
show("explode_native", lambda: struct.pack('<?', Exploding()))


# The default object (no __bool__/__len__) is always truthy.
show("plain_obj", lambda: struct.pack('?', object()).hex())


# The truth byte survives a round trip through unpack.
show("roundtrip", lambda: struct.unpack('?', struct.pack('?', Sized(0))))
