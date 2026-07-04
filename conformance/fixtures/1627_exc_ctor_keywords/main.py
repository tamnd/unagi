def s(tag):
    print("eval", tag)
    return tag


def show(label, fn):
    try:
        fn()
    except TypeError as e:
        print(label, "->", e)


# A builtin exception takes no keywords: the keyword is rejected only after
# every argument value has been evaluated, positional then keyword.
show("keyword", lambda: ValueError(s("pos"), x=s("kw")))

# The same rejection reaches the raise statement.
try:
    raise KeyError(reason="bad")
except TypeError as e:
    print("raise ->", e)

# The keyword rejection is spelled with the bare class name.
show("keyonly", lambda: RuntimeError(k=1))
show("group", lambda: ExceptionGroup("g", [ValueError()], k=1))

# The argument-assembly errors keep their normal precedence, spelled against
# the class: a keyword merge outranks the lone-star conversion, which outranks
# the key-stringness check, which outranks the takes-no-keyword rejection.
show("dup", lambda: ValueError(*[1], a=1, **{"a": 2}))
show("badmap", lambda: ValueError(**[1, 2]))
show("star-noniter", lambda: ValueError(*5, k=3))
show("nonstrkey", lambda: ValueError(**{1: 2}))

# A ** that merges to nothing leaves a plain construction untouched.
print("emptykw:", ValueError("ok", **{}).args)
# A star spreads into positional args and an empty ** adds no keywords.
print("star-empty:", ValueError(*["a", "b"], **{}).args)
