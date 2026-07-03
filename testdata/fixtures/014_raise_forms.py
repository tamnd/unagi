try:
    raise ValueError("boom")
except ValueError as e:
    print("caught:", e)
try:
    raise RuntimeError
except RuntimeError as e:
    print("bare class:", repr(e))
try:
    raise KeyError("missing")
except KeyError as e:
    print("keyerror str:", e)
    print("keyerror repr:", repr(e))
try:
    raise TypeError(1, 2)
except TypeError as e:
    print("two args:", e)
    print("two args repr:", repr(e))
def fail():
    raise ValueError("from fail")
try:
    fail()
except ValueError as e:
    print("from function:", e)
try:
    try:
        raise ValueError("inner")
    except ValueError:
        print("re-raising")
        raise
except ValueError as e:
    print("outer got:", e)
try:
    try:
        raise KeyError("k")
    except ValueError:
        print("not this one")
except KeyError as e:
    print("propagated:", e)
