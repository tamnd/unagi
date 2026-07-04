class C:
    def m(self, *args, **kw):
        return (args, sorted(kw.items()))


c = C()

# A method call may combine * unpacking with a literal keyword.
print(c.m(*[1, 2], k=3))
# A lone ** mapping spreads into keywords.
print(c.m(**{"a": 1, "b": 2}))
# Positional literals, a literal keyword, and a ** mapping all merge in order.
print(c.m(1, 2, x=9, **{"y": 10}))
# A * pack and a ** mapping combine.
print(c.m(*[7], p=1, **{"q": 2}))
# The positional pack keeps source order across a star in the middle.
print(c.m(0, *[1, 2], 3, key="v"))


def show(label, fn):
    try:
        fn()
    except TypeError as e:
        print(label, "->", e)


# A duplicate keyword between a literal and a ** mapping is caught at merge
# time, spelled against the module-qualified method for a user class.
show("dup", lambda: c.m(a=1, **{"a": 2}))
# A ** part that is not a mapping is rejected in argument position.
show("badmap", lambda: c.m(**[1, 2]))
# A lone * over a non-iterable is rejected when the call converts it.
show("starnoniter", lambda: c.m(*5, k=1))

# The same errors on a builtin method spell the bare type.method(), no module.
show("builtin-dup", lambda: "x".join(a=1, **{"a": 2}))
show("builtin-badmap", lambda: "x".join(**[1, 2]))
show("builtin-starnoniter", lambda: "x".join(*5, k=1))

# A builtin method that takes no keywords still rejects them, after the merge
# parts assemble cleanly.
show("builtin-nokw", lambda: "-".join(*[["1", "2"]], sep=","))
