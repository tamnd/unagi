# The six str methods that accept keyword arguments on CPython 3.14.
print("a b c".split(sep=" "))
print("a b c".split(" ", maxsplit=1))
print("a b c".split(maxsplit=1))
print("a-b-c".rsplit(sep="-", maxsplit=1))
print("x\ny\n".splitlines(keepends=True))
print("data".encode(encoding="ascii", errors="strict"))
print("data".encode(errors="replace"))
print("a\tb".expandtabs(tabsize=4))
print("banana".replace("a", "o", count=2))

# The arg-clinic errors match CPython.
def err(f):
    try:
        f()
    except TypeError as e:
        print(e)

err(lambda: "a".split(bad=1))
err(lambda: "a b".split(" ", sep=" "))
err(lambda: "a".splitlines(bad=1))
err(lambda: "a".encode(bad=1))
err(lambda: "aa".replace("a", count=1))
err(lambda: "a".count("a", start=0))
