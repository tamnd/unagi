# A memoryview binds its methods as readable instance attributes, mv.tobytes
# reads back and calls the same as mv.tobytes(), the buffer analog of the way
# every builtin binds its methods off an instance. The read answers on a released
# view too since the wrapper lives on the type; only a call through it raises.


def show(label, fn):
    try:
        print(label, repr(fn()))
    except Exception as e:
        print(label, "ERR", type(e).__name__, str(e))


mv = memoryview(bytearray(b"abc"))

# hasattr answers True across the implemented method surface.
names = ["tobytes", "tolist", "hex", "cast", "toreadonly", "release"]
print("has:", [n for n in names if hasattr(mv, n)])

# The bound reads call through to the methods.
show("tobytes", lambda: mv.tobytes())
show("tolist", lambda: mv.tolist())
show("hex", lambda: mv.hex())
show("cast tolist", lambda: mv.cast("b").tolist())
show("toreadonly ro", lambda: mv.toreadonly().readonly)

# A method reads back off a released view too; only a call raises.
r = memoryview(bytearray(b"x"))
r.release()
print("released has tobytes:", hasattr(r, "tobytes"))
show("released release()", lambda: r.release())
show("released tobytes()", lambda: r.tobytes())
