def show(label, fn):
    try:
        print(label, "->", repr(fn()))
    except Exception as e:
        print(label, "->", type(e).__name__, e)


def released():
    m = memoryview(bytearray(b"abcd"))
    m.release()
    return m


show("release returns", lambda: memoryview(b"ab").release())
show("double release", lambda: (lambda m: (m.release(), m.release())[1])(memoryview(b"ab")))
show("len after", lambda: len(released()))
show("tolist after", lambda: released().tolist())
show("index after", lambda: released()[0])
show("format after", lambda: released().format)
show("itemsize after", lambda: released().itemsize)
show("nbytes after", lambda: released().nbytes)
show("shape after", lambda: released().shape)
show("strides after", lambda: released().strides)
show("ndim after", lambda: released().ndim)
show("readonly after", lambda: released().readonly)
show("obj after", lambda: released().obj)
show("tobytes after", lambda: released().tobytes())
show("hex after", lambda: released().hex())
show("iter after", lambda: list(released()))
show("contains after", lambda: 1 in released())
show("cast after", lambda: released().cast("i"))
show("slice after", lambda: released()[0:2])
show("eq after (bytes)", lambda: released() == b"abcd")
show("eq after (mv)", lambda: memoryview(b"ab") == released())


def setitem_after():
    m = released()
    m[0] = 65
    return "ok"


def hash_after():
    return hash(released())


show("setitem after", setitem_after)
show("hash after", hash_after)


# __enter__ returns the view, the with-block releases it on the way out.
def enter_returns():
    m = memoryview(b"ab")
    return m.__enter__().tolist()


def ctx_releases():
    m = memoryview(bytearray(b"xy"))
    with m as v:
        inner = v.tolist()
    try:
        len(m)
        state = "still-open"
    except ValueError:
        state = "released"
    return inner, state


def exit_returns():
    return memoryview(b"ab").__exit__(None, None, None)


show("enter returns", enter_returns)
show("ctx releases", ctx_releases)
show("exit returns", exit_returns)


# release frees only this view; the underlying buffer stays usable.
def underlying_usable():
    ba = bytearray(b"abc")
    m = memoryview(ba)
    m.release()
    ba[0] = 90
    return bytes(ba)


show("underlying usable", underlying_usable)


# a fresh view over the same buffer after a release works normally.
def reopen():
    ba = bytearray(b"abc")
    memoryview(ba).release()
    return memoryview(ba).tolist()


show("reopen", reopen)
