def show(label, fn):
    try:
        print(label, "=>", repr(fn()))
    except Exception as e:
        print(label, "=>", type(e).__name__, ":", e)


# fromkeys is a classmethod, so reading it off a plain dict reports the dict type
# through __self__ and qualifies __qualname__ the way CPython inherits it, while
# __name__ stays bare and the callable still builds a fresh dict. An ordinary dict
# method read still binds the instance, so only fromkeys is steered to the class.
print("== fromkeys read off a plain dict binds the type ==")
print("{}.fromkeys.__self__:", {}.fromkeys.__self__)
print("{}.fromkeys.__qualname__:", {}.fromkeys.__qualname__)
print("{}.fromkeys.__name__:", {}.fromkeys.__name__)
d = {"a": 1, "b": 2}
print("populated dict fromkeys.__self__:", d.fromkeys.__self__)

print("== an ordinary dict method read still binds the instance ==")
print("{'x':1}.get.__self__:", {"x": 1}.get.__self__)
e = {"y": 2}
print("e.setdefault.__self__ is e:", e.setdefault.__self__ is e)
print("e.keys.__self__ is e:", e.keys.__self__ is e)

print("== the inherited classmethod still builds a fresh dict ==")
print("{}.fromkeys(['a', 'b']):", {}.fromkeys(["a", "b"]))
print("{'z':9}.fromkeys(['a', 'b'], 0):", {"z": 9}.fromkeys(["a", "b"], 0))
g = {}.fromkeys
print("g = {}.fromkeys; g('xy'):", g("xy"))
