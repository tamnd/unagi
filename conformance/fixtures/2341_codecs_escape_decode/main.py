# escape_decode is the _codecs C accelerator that reads the bytes backslash
# escapes (the named ones, octal, \x, the quote and line-continuation forms) back
# to raw bytes, the inverse of escape_encode. A bad \x escape routes through the
# strict/ignore/replace handling inline, a trailing backslash raises, and the
# first invalid or overflowing-octal escape emits a DeprecationWarning. This pins
# all of it against CPython, capturing the warnings deterministically so stderr
# stays clean and the warning text is checked on stdout.
import codecs
import warnings


def run(arg, errors=None):
    with warnings.catch_warnings(record=True) as caught:
        warnings.simplefilter("always")
        try:
            r = codecs.escape_decode(arg) if errors is None else codecs.escape_decode(arg, errors)
            msgs = [str(x.message) for x in caught]
            print(repr(arg), errors, "->", r, msgs)
        except Exception as e:
            print(repr(arg), errors, "-> EXC", type(e).__name__ + ":", e)


# Empty and the raw single-byte passthrough (every byte but backslash).
run(b"")
run(bytearray())
for b in range(256):
    bb = bytes([b])
    if bb != b"\\":
        got = codecs.escape_decode(bb + b"0")
        if got != (bb + b"0", 2):
            print("raw-mismatch", b, got)
print("raw-passthrough ok")

# The recognized escape catalog.
run(b"[\\\n]")
run(br"[\a\b\t\n\v\f\r]")
run(b"[\\'\\\"]")
run(br"[\\]")
run(br"[\7\41\101\x41]")
run(br"[\78\418\1010\x410]")
run(br"\0")
run(br"\08")

# Invalid escapes emit the first-only deprecation warning.
run(br"\z")
run(br"\8")
run(br"\9")
run(b"\\\xfa")
run(b"\\\x01")
run(br"\z\9\Q")

# Octal overflow warns and truncates to a byte.
run(br"\400")
run(br"\501")
run(br"\777")
run(br"\501\777")
run(br"\77")

# Bad \x through each handler, plus the trailing-backslash and unknown-handler
# errors.
run(br"\x")
run(br"[\x]")
run(br"\x0")
run(br"[\x]\x", "ignore")
run(br"[\x]\x", "replace")
run(br"[\x0]\x0", "ignore")
run(br"\xg", "ignore")
run(br"\xg", "replace")
run(br"\x4g", "ignore")
run(br"\x\z", "ignore")
run(br"\x\501", "ignore")
run(b"ab\\", "ignore")
run(br"\x", "backslashreplace")
run(br"\x", "bogus")
