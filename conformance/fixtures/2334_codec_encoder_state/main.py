# The MultibyteIncrementalEncoder getstate/setstate state protocol. getstate
# packs the encoder state into a single integer the way CPython does: a one-byte
# pending count, the pending combining base as UTF-8, then the codec state bytes,
# laid out little-endian. A per-unit codec that buffers a combining base
# (euc_jis_2004) and a shift-state codec that carries a designation (iso2022_jp)
# both round-trip through setstate. The validation matches CPython too.
import codecs

# euc_jis_2004 holds a combining base pending until the next code point decides
# whether it combines, so its state is a pending buffer.
buf = codecs.getincrementalencoder("euc_jis_2004")()
print("init", buf.getstate())
buf.encode("æ")
print("pending ae", buf.getstate())
buf.encode("̀")
print("after combine", buf.getstate())

# Reload the pending state into a fresh encoder and finish the sequence.
buf.encode("æ")
saved = buf.getstate()
reload = codecs.getincrementalencoder("euc_jis_2004")()
reload.setstate(saved)
print("replayed", reload.encode("̀"))

# iso2022_jp keeps a designation state without buffering a rune.
non = codecs.getincrementalencoder("iso2022_jp")()
print("iso init", non.getstate())
print("iso z", non.encode("z"))
en_state = non.getstate()
print("iso hira", non.encode("あ"))
jp_state = non.getstate()
non.setstate(jp_state)
print("iso replay", non.encode("あ"))
non.setstate(en_state)
print("iso ascii", non.encode("z"))

# Validation: a negative int, an over-large pending count and an invalid pending
# buffer each raise the way CPython does.
e = codecs.getincrementalencoder("euc_jp")()
cases = {
    "neg": -1,
    "size": int.from_bytes(b"\x09" + b"\x00" * 16, "little"),
    "bytes": int.from_bytes(b"\x01\xff" + b"\x00" * 8, "little"),
}
for label, value in cases.items():
    try:
        e.setstate(value)
    except (OverflowError, UnicodeError) as exc:
        print(label, type(exc).__name__, exc)
