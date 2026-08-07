def show(label, fn):
    try:
        print(label, repr(fn()))
    except Exception as ex:
        print(label, "ERR", type(ex).__name__, str(ex))


class Idx:
    def __index__(self):
        return 65


class Bad:
    def __index__(self):
        return "x"


class Huge:
    def __index__(self):
        return 300


def ba_append(v):
    ba = bytearray(b"z")
    ba.append(v)
    return bytes(ba)


def ba_insert(v):
    ba = bytearray(b"ac")
    ba.insert(1, v)
    return bytes(ba)


def ba_setitem(v):
    ba = bytearray(b"abc")
    ba[1] = v
    return bytes(ba)


# The bytearray mutators that take a byte value now feed it through __index__ the
# way CPython does, so an object spelling __index__ stores as its value, a bool
# counts, a value outside range(0, 256) is the byte-range ValueError and a bad
# __index__ return propagates its non-int TypeError.
show("append-idx", lambda: ba_append(Idx()))
show("append-bool", lambda: ba_append(True))
show("append-huge", lambda: ba_append(Huge()))
show("append-bad", lambda: ba_append(Bad()))
show("append-float", lambda: ba_append(1.5))
show("insert-idx", lambda: ba_insert(Idx()))
show("insert-bad", lambda: ba_insert(Bad()))
show("setitem-idx", lambda: ba_setitem(Idx()))
show("setitem-huge", lambda: ba_setitem(Huge()))
show("setitem-bad", lambda: ba_setitem(Bad()))

# The search methods take a byte value the same way, on bytes and bytearray, an
# __index__ object matching its byte, a bad return propagating and a non-integer
# with no __index__ the argument-should-be-integer TypeError.
show("count-idx", lambda: b"AAA".count(Idx()))
show("count-bad", lambda: b"AAA".count(Bad()))
show("count-str", lambda: b"AAA".count("A"))
show("find-idx", lambda: b"ABCA".find(Idx()))
show("index-idx", lambda: b"ABCA".index(Idx()))
show("rfind-idx", lambda: b"ABCA".rfind(Idx()))
show("rindex-idx", lambda: b"ABCA".rindex(Idx()))
show("ba-count-idx", lambda: bytearray(b"AAA").count(Idx()))
show("ba-find-idx", lambda: bytearray(b"ABC").find(Idx()))

# Membership takes a byte value through __index__ too, but unlike the search
# methods a bad __index__ return does not propagate: CPython clears it and falls
# through, so a non-int return and no __index__ reach the same bytes-like
# TypeError, while a valid out-of-range value is still the byte-range ValueError.
show("in-idx-true", lambda: Idx() in b"ABC")
show("in-idx-false", lambda: Idx() in b"BCD")
show("in-bad", lambda: Bad() in b"ABC")
show("in-huge", lambda: Huge() in b"ABC")
show("ba-in-idx", lambda: Idx() in bytearray(b"ABC"))
show("ba-in-bad", lambda: Bad() in bytearray(b"ABC"))
