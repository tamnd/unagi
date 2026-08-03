# _random.Random is a C type, so its methods report CPython's Argument Clinic
# messages: the positional counts, the module-qualified no-keyword errors, and
# getrandbits reading its argument as a uint64_t.
import _random


def show(label, fn):
    try:
        fn()
        print(label, "(no error)")
    except Exception as e:
        print(label, type(e).__name__, str(e))


r = _random.Random(1)
# Constructor arity and keywords.
show("init 2 args", lambda: _random.Random(1, 2))
show("init kw", lambda: _random.Random(a=1))
# random takes no arguments.
show("random 1", lambda: r.random(1))
show("random kw", lambda: r.random(x=1))
# seed takes at most one.
show("seed 2", lambda: r.seed(1, 2))
show("seed kw", lambda: r.seed(x=1))
# getrandbits takes exactly one, read as a uint64_t.
show("getrandbits 0", lambda: r.getrandbits())
show("getrandbits 2", lambda: r.getrandbits(1, 2))
show("getrandbits kw", lambda: r.getrandbits(k=1))
show("getrandbits neg", lambda: r.getrandbits(-1))
show("getrandbits huge", lambda: r.getrandbits(1 << 64))
show("getrandbits float", lambda: r.getrandbits(1.5))
# getstate takes no arguments, setstate exactly one.
show("getstate 1", lambda: r.getstate(1))
show("setstate 0", lambda: r.setstate())
show("setstate 2", lambda: r.setstate(1, 2))

# getrandbits(0) is a valid zero and the boundaries still draw a real int.
print(r.getrandbits(0))
print(0 <= r.getrandbits(1) <= 1)
print(r.getrandbits(64).bit_length() <= 64)
