import array
import weakref


def show(label, o):
    try:
        r = weakref.ref(o)
        print(label, "=> ref ok, deref matches", r() is o)
    except TypeError as e:
        print(label, "=> TypeError:", e)


# array.array and memoryview declare a __weakref__ slot the way CPython's C
# types do, so they can be weakly referenced; the other builtin containers with
# no slot still cannot.
print("== which builtins accept a weak reference ==")
a = array.array("i", [1, 2, 3])
mv = memoryview(b"abcd")
show("array", a)
show("memoryview", mv)
show("frozenset", frozenset({1, 2}))
show("list", [1, 2])
show("dict", {1: 2})
show("tuple", (1, 2))
show("bytearray", bytearray(b"ab"))
show("bytes", b"ab")

print("== the reference dereferences to the live object ==")
r = weakref.ref(a)
print("deref is a", r() is a)
print("count", weakref.getweakrefcount(a))
rm = weakref.ref(mv)
print("mv deref is mv", rm() is mv)
print("mv count", weakref.getweakrefcount(mv))

print("== a proxy forwards to the array ==")
p = weakref.proxy(a)
print("proxy len", len(p))
print("proxy elem", p[1])
p.append(4)
print("array after proxy append", a.tolist())

print("== a callback registers without raising ==")
r2 = weakref.ref(a, lambda ref: None)
print("ref-with-callback deref", r2() is a)

print("== a WeakValueDictionary can hold an array or memoryview value ==")
d = weakref.WeakValueDictionary()
d["arr"] = a
d["mv"] = mv
print("value array", d["arr"].tolist())
print("value memoryview", bytes(d["mv"]))
print("wvd len", len(d))
