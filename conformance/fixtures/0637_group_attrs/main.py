g = ExceptionGroup("group msg", [ValueError("a"), KeyError("b")])
print("message:", repr(g.message))
print("exceptions:", g.exceptions)
print("count:", len(g.exceptions))
print("args:", g.args)

be = BaseExceptionGroup("bmsg", [KeyError("only")])
print("bmessage:", repr(be.message))
print("bexc:", be.exceptions)

try:
    ValueError("x").message
except AttributeError as e:
    print("plain message:", e)
try:
    ValueError("x").exceptions
except AttributeError as e:
    print("plain exceptions:", e)

try:
    raise ExceptionGroup("boom", [ValueError("a"), ValueError("c"), ValueError("d")])
except* ValueError as caught:
    print("caught message:", caught.message)
    print("caught count:", len(caught.exceptions))
    for sub in caught.exceptions:
        print("  sub:", repr(sub))
