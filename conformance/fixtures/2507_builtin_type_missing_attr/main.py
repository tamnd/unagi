import collections


def show(label, fn):
    try:
        fn()
        print(label, "=> NO ERROR")
    except AttributeError as e:
        print(label, "=>", e)
    except Exception as e:
        print(label, "=>", type(e).__name__, ":", e)


# A builtin type reports a missing attribute as a type object, the way CPython's
# type getattro does: "type object 'int' has no attribute 'nope'". A type read
# that resolves (a classmethod, an unbound method, a dunder) is unaffected, and a
# module-qualified type reports its bare __name__ tail.
print("== a missing attribute on a builtin type names the type ==")
show("int", lambda: int.nope)
show("float", lambda: float.nope)
show("complex", lambda: complex.nope)
show("bool", lambda: bool.nope)
show("str", lambda: str.nope)
show("bytes", lambda: bytes.nope)
show("bytearray", lambda: bytearray.nope)
show("list", lambda: list.nope)
show("tuple", lambda: tuple.nope)
show("dict", lambda: dict.nope)
show("set", lambda: set.nope)
show("frozenset", lambda: frozenset.nope)
show("range", lambda: range.nope)
show("type", lambda: type.nope)
show("object", lambda: object.nope)

print("== a module-qualified type reports its bare name ==")
show("deque", lambda: collections.deque.nope)
show("OrderedDict", lambda: collections.OrderedDict.nope)

print("== a resolving read on a builtin type is unaffected ==")
print("int.from_bytes", int.from_bytes(b"\x01\x02", "big"))
print("float.fromhex", float.fromhex("0x1p4"))
print("str.upper unbound", str.upper("ab"))
print("int.bit_length unbound", int.bit_length(255))
print("dict.fromkeys", dict.fromkeys(["a", "b"]))
print("list.append name", list.append.__name__)

print("== an instance still reports as its own type ==")
show("(5).nope", lambda: (5).nope)
show("'x'.nope", lambda: "x".nope)
show("[].nope", lambda: [].nope)
