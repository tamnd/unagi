import binascii


def show(label, fn):
    try:
        print(label, "=>", repr(fn()))
    except Exception as e:
        print(label, "=>", type(e).__name__, ":", e)


# A plain builtin function carries a __call__ that forwards to it, the way
# callable introspection and unittest's assertHasAttr(func, '__call__') read it.
show("len_has_call", lambda: hasattr(len, "__call__"))
show("len_call_invoke", lambda: len.__call__([1, 2, 3]))
show("callable_len_call", lambda: callable(len.__call__))

# A native-module builtin function is the same, so binascii.b2a_hex round trips
# through its own __call__.
show("b2a_has_call", lambda: hasattr(binascii.b2a_hex, "__call__"))
show("b2a_call_invoke", lambda: binascii.b2a_hex.__call__(b"AB"))

# A builtin type constructor answers __call__ too, forwarding to construction,
# so int.__call__(5) builds the same value int(5) does.
show("int_has_call", lambda: hasattr(int, "__call__"))
show("int_call_invoke", lambda: int.__call__(5))
show("str_call_invoke", lambda: str.__call__(42))

# A bound builtin method exposes __call__ as well.
show("upper_call", lambda: "abc".upper.__call__())
show("append_has_call", lambda: hasattr([].append, "__call__"))

# The wrapper is itself callable, so a chained __call__.__call__ still forwards.
show("chained_call", lambda: len.__call__.__call__([1, 2]))

# Wrong arity through the wrapper raises the underlying function's TypeError.
show("len_call_no_args", lambda: len.__call__())
