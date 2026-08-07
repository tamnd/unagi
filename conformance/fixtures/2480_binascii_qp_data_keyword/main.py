import binascii


def show(label, fn):
    try:
        print(label, "=>", repr(fn()))
    except Exception as e:
        print(label, "=>", type(e).__name__, ":", e)


# data is a positional-or-keyword argument for both codecs, the call
# test_qp opens with, so passing it by name decodes and encodes the same.
show("a2b_data_kw", lambda: binascii.a2b_qp(data=b"caf=E9", header=False))
show("a2b_pos", lambda: binascii.a2b_qp(b"caf=E9"))
show("b2a_data_kw", lambda: binascii.b2a_qp(data=b"caf\xe9"))
show("b2a_all_kw", lambda: binascii.b2a_qp(data=b"a b", quotetabs=True, istext=False, header=True))

# With no data the argument clinic names the missing required argument.
show("a2b_no_args", lambda: binascii.a2b_qp())
show("b2a_no_args", lambda: binascii.b2a_qp())

# data given by name and position is the by-name-and-position error.
show("a2b_dup", lambda: binascii.a2b_qp(b"x", data=b"y"))
show("b2a_dup", lambda: binascii.b2a_qp(b"x", data=b"y"))

# A non-string keyword is rejected before parsing, the SF bug 534347 guard.
show("a2b_int_kw", lambda: binascii.a2b_qp(b"", **{1: 1}))

# Too many positionals still reports the arity.
show("a2b_3pos", lambda: binascii.a2b_qp(b"", False, 3))

# An unknown keyword still names itself.
show("a2b_bogus_kw", lambda: binascii.a2b_qp(b"", bogus=True))
