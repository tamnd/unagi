def show(label, fn):
    try:
        print(label, "=>", repr(fn()))
    except Exception as e:
        print(label, "=>", type(e).__name__, ":", e)


# A bound builtin instance method reports its receiver through __self__, the way
# CPython's bound method does. The receiver is the answer for every type, so a
# number, a string, a container and a buffer each hand back the object the method
# was read off. __name__ stays the bare method name.
print("== a bound number method binds its number ==")
n = 255
print("(255).to_bytes.__self__:", n.to_bytes.__self__, "| is n:", n.to_bytes.__self__ is n)
print("(255).bit_length.__self__:", (255).bit_length.__self__)
print("(255).conjugate.__self__:", (255).conjugate.__self__)
print("True.bit_length.__self__:", True.bit_length.__self__)
f = 1.5
print("(1.5).is_integer.__self__:", f.is_integer.__self__, "| is f:", f.is_integer.__self__ is f)
print("(1.5).as_integer_ratio.__self__:", (1.5).as_integer_ratio.__self__)
print("(1.5).hex.__self__:", (1.5).hex.__self__)
c = 3 + 4j
print("(3+4j).conjugate.__self__:", c.conjugate.__self__, "| is c:", c.conjugate.__self__ is c)

print("== a bound container or string method binds its object ==")
s = "abc"
print("'abc'.upper.__self__:", repr(s.upper.__self__), "| is s:", s.upper.__self__ is s)
lst = [1, 2]
print("[1,2].append.__self__:", lst.append.__self__, "| is lst:", lst.append.__self__ is lst)
d = {"k": 1}
print("{'k':1}.get.__self__:", d.get.__self__, "| is d:", d.get.__self__ is d)
st = {1, 2}
print("{1,2}.add.__self__:", st.add.__self__, "| is st:", st.add.__self__ is st)
print("b'xy'.hex.__self__:", b"xy".hex.__self__)
ba = bytearray(b"xy")
print("bytearray(b'xy').hex.__self__:", ba.hex.__self__, "| is ba:", ba.hex.__self__ is ba)
print("(1,2,1).count.__self__:", (1, 2, 1).count.__self__)

print("== __name__ stays the bare method name ==")
print("(255).to_bytes.__name__:", (255).to_bytes.__name__)
print("[1,2].append.__name__:", [1, 2].append.__name__)
print("'abc'.upper.__name__:", "abc".upper.__name__)

print("== a bound method still calls ==")
print("(255).to_bytes(2, 'big'):", (255).to_bytes(2, "big"))
print("(1.5).is_integer():", (1.5).is_integer())
print("'abc'.upper():", "abc".upper())
lst2 = [1]
lst2.append(9)
print("append result:", lst2)
