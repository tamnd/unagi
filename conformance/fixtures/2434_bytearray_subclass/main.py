class BA(bytearray):
    pass


def show(label, e):
    try:
        v = e()
        print(label, repr(v), type(v).__name__)
    except Exception as ex:
        print(label, "ERR", type(ex).__name__, ex)


# A bytearray subclass constructs from the same argument forms as the base and the
# result carries the subclass type while repr/str/format all name the subclass.
show("from-bytes", lambda: BA(b"abc"))
show("from-int", lambda: BA(3))
show("from-iterable", lambda: BA([65, 66, 67]))
show("from-str", lambda: BA("hello", "utf-8"))
show("empty", lambda: BA())
show("str", lambda: str(BA(b"xy")))
show("format-empty", lambda: format(BA(b"xy"), ""))
show("format-spec", lambda: format(BA(b"xy"), "x"))

# Mutating methods operate on the subclass in place and keep the subclass type.
def mutate():
    b = BA(b"abc")
    b.append(100)
    b.extend(b"ef")
    b.insert(0, 90)
    b[1] = 89
    del b[2]
    b.reverse()
    return b


show("mutate", mutate)

# pop returns an int and leaves the shrunk subclass behind.
def pop_case():
    b = BA(b"abcd")
    x = b.pop()
    return (x, b)


show("pop", pop_case)

# Read methods inherit and return their native types, not the subclass.
show("upper", lambda: BA(b"abc").upper())
show("split", lambda: BA(b"a b c").split())
show("hex", lambda: BA(b"\x01\x02").hex())
show("find", lambda: BA(b"abcabc").find(b"c"))
show("decode", lambda: BA(b"hi").decode())

# Operators: concat and repeat build new bytearrays, in place ops keep the subtype.
show("add", lambda: BA(b"ab") + b"cd")
show("mul", lambda: BA(b"ab") * 2)


def iadd():
    b = BA(b"ab")
    b += b"cd"
    return b


def imul():
    b = BA(b"ab")
    b *= 2
    return b


show("iadd", iadd)
show("imul", imul)

# Comparison, membership, iteration, len, indexing and slicing read through.
show("eq", lambda: BA(b"abc") == b"abc")
show("contains", lambda: 98 in BA(b"abc"))
show("iter", lambda: list(BA(b"abc")))
show("len", lambda: len(BA(b"abc")))
show("index", lambda: BA(b"abc")[1])
show("slice", lambda: BA(b"abcde")[1:4])

# The buffer protocol works: memoryview over the subclass writes through to it.
def mv_write():
    b = BA(b"abc")
    m = memoryview(b)
    m[0] = 122
    return b


show("memoryview-write", mv_write)

# A bytearray subclass is mutable, so it is unhashable like the base.
show("unhash", lambda: hash(BA(b"abc")))

# bool and isinstance behave through the subclass.
show("bool-empty", lambda: bool(BA()))
show("bool-full", lambda: bool(BA(b"x")))
show("isinstance-bytearray", lambda: isinstance(BA(b"x"), bytearray))
show("isinstance-ba", lambda: isinstance(bytearray(b"x"), BA))

# The plain base is unchanged.
show("plain", lambda: bytearray(b"abc"))
