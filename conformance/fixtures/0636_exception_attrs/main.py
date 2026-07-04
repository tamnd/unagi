try:
    try:
        raise ValueError("inner")
    except ValueError:
        raise RuntimeError("outer") from None
except RuntimeError as r:
    print("args:", r.args)
    print("cause:", repr(r.__cause__))
    print("context:", repr(r.__context__))
    print("suppress:", r.__suppress_context__)

e = ValueError("x")
print("fresh tb:", repr(e.__traceback__))
try:
    print(e.__notes__)
except AttributeError as a:
    print("no notes:", a)
e.add_note("first")
e.add_note("second")
print("notes:", e.__notes__)

c = KeyError("k")
e.__cause__ = c
print("set cause:", repr(e.__cause__), "suppress:", e.__suppress_context__)
e.__cause__ = None
print("clear cause:", repr(e.__cause__), "suppress:", e.__suppress_context__)

e.__context__ = c
print("set context:", repr(e.__context__))
e.__suppress_context__ = False
print("reset suppress:", e.__suppress_context__)

try:
    e.__cause__ = 5
except TypeError as t:
    print("cause err:", t)
try:
    e.__context__ = 5
except TypeError as t:
    print("context err:", t)
try:
    e.__suppress_context__ = "yes"
except TypeError as t:
    print("suppress err:", t)
