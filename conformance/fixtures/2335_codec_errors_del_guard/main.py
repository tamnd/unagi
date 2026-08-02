# The incremental encoder and decoder expose errors as a managed attribute the
# way CPython's C getset does: it reads and reassigns, but deleting it raises
# AttributeError rather than dropping the handler and leaving the codec in a
# degenerate state (bug #3305 territory). A plain instance attribute set by the
# caller still deletes normally, so only the managed handler is protected.
import codecs

for label, make in (
    ("encoder", codecs.getincrementalencoder("euc_jp")),
    ("decoder", codecs.getincrementaldecoder("euc_jp")),
):
    obj = make()
    print(label, "default", repr(obj.errors))
    obj.errors = "replace"
    print(label, "reassigned", repr(obj.errors))
    try:
        del obj.errors
    except AttributeError as exc:
        print(label, "del errors", type(exc).__name__, exc)
    print(label, "still", repr(obj.errors))
    obj.tag = 7
    del obj.tag
    print(label, "user attr deletes")
