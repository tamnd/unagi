# The MultibyteIncrementalDecoder getstate/setstate state protocol. getstate
# reports the pending bytes and the codec state integer, setstate restores them
# so a decoder can be rewound to an earlier point, and the raw state integer of a
# stateless codec round-trips verbatim. The validation rejects a non-tuple
# argument, a wrong tuple shape and an over-large pending buffer the way CPython
# does.
import codecs

d = codecs.getincrementaldecoder("euc_jp")()

# A complete sequence leaves no pending bytes.
print(d.decode(b"\xa4\xa6"))
print("state after full", d.getstate())

# The first half of a two-byte sequence is held pending.
print("half", repr(d.decode(b"\xa4")))
pending, flags = d.getstate()
print("pending", pending, flags)

# Rewind to the pending half and finish the sequence again.
print("second", d.decode(b"\xa6"))
d.setstate((pending, flags))
print("replayed", d.decode(b"\xa6"))

# The state integer of a stateless codec is opaque and round-trips unchanged.
d.setstate((b"abc", 123456789))
print("roundtrip", d.getstate())

# A shift-state codec carries its designation state through getstate.
j = codecs.getincrementaldecoder("iso2022_jp")()
print("iso2022", j.decode(b"\x1b$B@$"))
js = j.getstate()
print("iso2022 state", js)
j.setstate(js)
print("iso2022 replay", j.decode(b"@$"))

# Validation: the argument must be a tuple of a bytes buffer and an int, and the
# pending buffer cannot exceed the pending cap.
for bad in (123, ("invalid", 0), (b"1234", "invalid"), (b"123456789", 0)):
    try:
        codecs.getincrementaldecoder("euc_jp")().setstate(bad)
    except (TypeError, UnicodeDecodeError) as exc:
        print("bad", type(exc).__name__, exc)

# Eight pending bytes are the largest buffer setstate accepts.
ok = codecs.getincrementaldecoder("euc_jp")()
ok.setstate((b"12345678", 0))
print("eight ok", ok.getstate())
