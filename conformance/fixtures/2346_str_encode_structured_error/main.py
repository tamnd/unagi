def show(label, fn):
    try:
        fn()
        print(label, "no error")
    except UnicodeEncodeError as e:
        print(label, "enc", e.encoding, "start", e.start, "end", e.end, "reason", e.reason)
        print("  object", ascii(e.object), "span", ascii(e.object[e.start:e.end]))
        print("  str", str(e))
        print("  is UnicodeError", isinstance(e, UnicodeError))


# single unencodable character
show("ascii-single", lambda: "a\xffb".encode("ascii"))
show("latin1-single", lambda: "aĀb".encode("latin-1"))

# a run of consecutive unencodable characters coalesces into one span
show("ascii-run", lambda: "a\xff\xffb".encode("ascii"))
show("ascii-run3", lambda: "a\xffĀ\U00010280b".encode("ascii"))
show("latin1-run", lambda: "aĀāb".encode("latin-1"))

# a normal character between two bad ones breaks the run into two errors
show("ascii-split", lambda: "a\xffZ\xffb".encode("ascii"))

# utf-8 only rejects lone surrogates; runs coalesce, a normal char breaks them
show("utf8-surr-single", lambda: "a\ud800b".encode("utf-8"))
show("utf8-surr-run", lambda: "a\ud800\ud801b".encode("utf-8"))
show("utf8-surr-split", lambda: "a\ud800Z\ud801b".encode("utf-8"))

# surrogatepass on a narrow codec cannot represent the surrogate and reports it
show("latin1-surrpass", lambda: "a\ud800\ud801b".encode("latin-1", "surrogatepass"))

# surrogateescape re-raises a non-escapable character, span over the raw run
show("ascii-srgesc", lambda: "a\U00010280\ud801b".encode("ascii", "surrogateescape"))
show("utf8-srgesc", lambda: "a\U00010280\ud801b".encode("utf-8", "surrogateescape"))

# a caught error can be re-raised and re-inspected
def reinspect():
    try:
        "\xff".encode("ascii")
    except UnicodeEncodeError as e:
        raise e


try:
    reinspect()
except UnicodeEncodeError as e:
    print("reraised", e.encoding, e.start, e.end, e.reason)
