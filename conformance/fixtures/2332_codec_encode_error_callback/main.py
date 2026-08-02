# Custom encode error handlers registered through codecs.register_error, driving
# the multibyte codec encode error-callback contract: the handler returns a
# (replacement, newpos) tuple, a str replacement is re-encoded through the codec
# while a bytes replacement is emitted verbatim, and newpos steers where the loop
# resumes (forward to skip, backward via a negative wrap). The bad-position and
# bad-return validation is exercised too. Both a per-unit codec (euc_jp) and the
# shift-state codecs (iso2022_jp, iso2022_kr, hz) run the callback so the
# replacement rejoins each codec's own state.
import codecs

# Two code points outside every CJK repertoire below, so each one routes to the
# handler while the surrounding ascii and mappable characters encode normally.
s = "xሴy\U0001F600z"


def to_hex(e):
    return ("<%04x>" % ord(e.object[e.start]), e.start + 1)


def to_bytes(e):
    return (b"\x00\xff", e.end)


def skip_next(e):
    # drop the bad char and the one after it, resuming two past the start
    return ("", e.start + 2)


def wrap_neg(e):
    # a negative newpos wraps from the end of the object; here it resolves back
    # to just past the bad char
    return ("!", e.start + 1 - len(e.object))


codecs.register_error("t.hex", to_hex)
codecs.register_error("t.bytes", to_bytes)
codecs.register_error("t.skip", skip_next)
codecs.register_error("t.wrap", wrap_neg)

for enc in ("euc_jp", "iso2022_jp", "iso2022_kr", "hz", "gb18030"):
    print(enc, "hex", codecs.encode(s, enc, "t.hex"))
    print(enc, "bytes", codecs.encode(s, enc, "t.bytes"))
    print(enc, "skip", codecs.encode(s, enc, "t.skip"))
    print(enc, "wrap", codecs.encode(s, enc, "t.wrap"))

# A str replacement is re-encoded through the codec, so a mappable replacement
# lands as that codec's own bytes and keeps the shift state consistent.
codecs.register_error("t.kana", lambda e: ("・", e.end))
print("iso2022_jp kana", codecs.encode("a" + "\U0001F600" + "b", "iso2022_jp", "t.kana"))


def out_of_range(e):
    return ("", 999)


def bad_pos_type(e):
    return ("", "nope")


def bad_replacement(e):
    return (123, e.end)


codecs.register_error("t.oor", out_of_range)
codecs.register_error("t.badpos", bad_pos_type)
codecs.register_error("t.badrep", bad_replacement)

for name in ("t.oor", "t.badpos", "t.badrep"):
    try:
        codecs.encode(s, "euc_jp", name)
    except (IndexError, TypeError) as exc:
        print("euc_jp", name, type(exc).__name__, exc)
