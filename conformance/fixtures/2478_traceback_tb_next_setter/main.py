def show(label, fn):
    try:
        print(label, "=>", repr(fn()))
    except Exception as e:
        print(label, "=>", type(e).__name__, ":", e)


def tb_of(exc_type, msg):
    try:
        raise exc_type(msg)
    except exc_type as e:
        return e.__traceback__


def walk(t):
    n = 0
    while t is not None:
        n += 1
        t = t.tb_next
    return n


tb = tb_of(ValueError, "x")
tb2 = tb_of(KeyError, "y")

# tb_next reads back and starts as None at the innermost frame.
show("has_tb_next", lambda: hasattr(tb, "tb_next"))
show("tb_next_start", lambda: tb.tb_next)


# Assigning None leaves it None, the trim unittest performs on its own frames.
def set_none():
    tb.tb_next = None
    return tb.tb_next


show("set_none", set_none)


# Assigning another traceback splices it in, and the chain walks through.
def splice():
    tb.tb_next = tb2
    return tb.tb_next is tb2


show("splice", splice)
show("chain_length", lambda: walk(tb))


# A self loop is refused, and so is any longer cycle.
def self_loop():
    tb.tb_next = tb


show("self_loop", self_loop)

# A non-traceback value is the expected-traceback TypeError.
show("set_int", lambda: setattr(tb, "tb_next", 5))
show("set_str", lambda: setattr(tb, "tb_next", "z"))

# The other three attributes stay read-only, each with CPython's wording.
show("set_lineno", lambda: setattr(tb, "tb_lineno", 5))
show("set_frame", lambda: setattr(tb, "tb_frame", None))
show("set_lasti", lambda: setattr(tb, "tb_lasti", 3))

# tb_next cannot be deleted, only assigned None.
show("del_next", lambda: delattr(tb, "tb_next"))

# After all that, the splice still holds and reads back the frames.
show("still_spliced", lambda: tb.tb_next is tb2)
show("final_length", lambda: walk(tb))
