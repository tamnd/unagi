# int.from_bytes accepts the bytes argument by keyword, alongside byteorder and
# signed, with CPython's duplicate and missing argument reporting.
def show(label, fn):
    try:
        print(label, "OK", fn())
    except Exception as e:
        print(label, type(e).__name__, e)


show("kw_bytes", lambda: int.from_bytes(bytes=b"\x01\x02", byteorder="big"))
show("kw_both_default_order", lambda: int.from_bytes(bytes=b"\x01\x02"))
show("kw_order_only_pos_bytes", lambda: int.from_bytes(b"\x01\x02", byteorder="little"))
show("kw_signed", lambda: int.from_bytes(b"\xff", byteorder="big", signed=True))
show("list_bytes_kw", lambda: int.from_bytes(bytes=[1, 2, 3], byteorder="big"))
show("dup_bytes", lambda: int.from_bytes(b"\x01", bytes=b"\x02"))
show("dup_order", lambda: int.from_bytes(b"\x01", "big", byteorder="little"))
show("missing_bytes", lambda: int.from_bytes(byteorder="big"))
show("missing_vs_badorder", lambda: int.from_bytes(byteorder="sideways"))
show("badbytes_vs_badorder", lambda: int.from_bytes(bytes=5, byteorder="sideways"))
show("unexpected_kw", lambda: int.from_bytes(b"\x01", nope=1))


class MyInt(int):
    pass


sub = MyInt.from_bytes(bytes=b"\x01\x02", byteorder="big")
print("subclass_kw_bytes", type(sub).__name__, sub)
