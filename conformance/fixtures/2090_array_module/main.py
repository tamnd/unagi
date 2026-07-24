import warnings
warnings.simplefilter('ignore')  # the 'u' typecode is deprecated in 3.14
import array


def show(label, fn):
    try:
        print(label, fn())
    except (TypeError, ValueError, OverflowError, IndexError) as ex:
        print(label, type(ex).__name__ + ':', ex)


print("typecodes", array.typecodes)
for tc in 'bBhHiIlLqQfd':
    a = array.array(tc, [1, 2, 3])
    print(tc, a.typecode, a.itemsize, a.tolist(), repr(a))

a = array.array('i', [10, 20, 30, 40])
print("index", a[0], a[-1])
print("slice", a[1:3].tolist())
a[1] = 99
print("setitem", a.tolist())
del a[0]
print("delitem", a.tolist())
a.append(7)
a.extend([8, 9])
a.insert(0, -1)
print("grow", a.tolist(), len(a))
print("pop", a.pop(), a.pop(0), a.tolist())
a.remove(99)
print("remove", a.tolist(), "idx", a.index(8), "count", a.count(7))
a.reverse()
print("reverse", a.tolist())
print("iter", list(iter(a)), "in", 8 in a, 100 in a)

print("eq", array.array('i', [1, 2]) == array.array('i', [1, 2]))
print("eq-cross", array.array('i', [1]) == array.array('l', [1]))
print("neq", array.array('i', [1, 2]) == array.array('i', [1, 3]))
print("concat", (array.array('i', [1, 2]) + array.array('i', [3])).tolist())
print("repeat", (array.array('i', [1, 2]) * 2).tolist())

d = array.array('i', [1, 2])
d += array.array('i', [9])
print("iadd", d.tolist())
d *= 2
print("imul", d.tolist())
d *= 0
print("imul0", d.tolist())

b = array.array('d', [1.5, 2.5])
print("double", b.tolist(), b.itemsize, repr(b))
u = array.array('u', 'hi')
print("unicode", u.tounicode(), repr(u))
u.fromunicode('!')
print("fromunicode", u.tounicode())

raw = array.array('i', [1, 2, 3]).tobytes()
c = array.array('i')
c.frombytes(raw)
print("bytes-roundtrip", c.tolist())
sw = array.array('h', [1, 256])
sw.byteswap()
print("byteswap", sw.tolist())
print("empty-repr", repr(array.array('i')))
print("bool", bool(array.array('i')), bool(array.array('i', [1])))

show("overflow", lambda: array.array('b', [200]))
show("bad-typecode", lambda: array.array('z', [1]))
show("str-init", lambda: array.array('i', 'ab'))
show("index-range", lambda: array.array('i', [1])[5])
show("unhashable", lambda: hash(array.array('i', [1])))
show("pop-empty", lambda: array.array('i').pop())
show("not-in", lambda: array.array('i', [1]).index(9))
show("frombytes-size", lambda: array.array('i').frombytes(b'ab'))
