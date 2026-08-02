def show(label, fn):
    try:
        fn()
        print(label, "no error")
    except UnicodeDecodeError as e:
        print(label, "enc", e.encoding, "start", e.start, "end", e.end, "reason", e.reason)
        print("  object", e.object, "span", e.object[e.start:e.end])
        print("  str", str(e))
        print("  is UnicodeError", isinstance(e, UnicodeError))


# single-byte spans
show("ascii", lambda: b"a\xffb".decode("ascii"))
show("latin1-ok", lambda: b"\xff".decode("latin-1"))
show("utf8-start", lambda: b"a\xffb".decode("utf-8"))
show("utf8-badcont", lambda: b"\xe0\x80".decode("utf-8"))

# multi-byte spans
show("utf8-trunc3", lambda: b"a\xe0\xa4".decode("utf-8"))
show("utf8-2of4", lambda: b"\xf0\x90".decode("utf-8"))

# the attributes read the same off a caught error regardless of handler kwarg
show("ascii-strict", lambda: bytes([0x80]).decode("ascii", "strict"))

# a caught error can be re-raised and re-inspected
def reinspect():
    try:
        b"\xff".decode("ascii")
    except UnicodeDecodeError as e:
        raise e


try:
    reinspect()
except UnicodeDecodeError as e:
    print("reraised", e.encoding, e.start, e.end, e.reason)
