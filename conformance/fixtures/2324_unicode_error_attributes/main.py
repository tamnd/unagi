# UnicodeDecodeError, UnicodeEncodeError and UnicodeTranslateError built from the
# structured constructor expose their arguments as attributes and render str()
# in CPython's codec-message form.
d = UnicodeDecodeError("utf-8", b"abcdef", 1, 2, "reason")
print("d_str", str(d))
print("d_attrs", d.encoding, d.object, d.start, d.end, d.reason)
print("d_args", d.args)
print("d_span", str(UnicodeDecodeError("utf-8", b"abcdef", 1, 4, "reason")))

e = UnicodeEncodeError("ascii", "a\xe9c", 1, 2, "why")
print("e_str", str(e))
print("e_attrs", e.encoding, e.object, e.start, e.end, e.reason)
print("e_bmp", str(UnicodeEncodeError("ascii", "a中c", 1, 2, "why")))
print("e_astral", str(UnicodeEncodeError("ascii", "a\U0001F600c", 1, 2, "why")))
print("e_span", str(UnicodeEncodeError("ascii", "abcdef", 1, 4, "why")))

t = UnicodeTranslateError("a\xe9c", 1, 2, "no")
print("t_str", str(t))
print("t_attrs", t.encoding, t.object, t.start, t.end, t.reason)
print("t_span", str(UnicodeTranslateError("abcdef", 1, 4, "no")))

# The subclass hierarchy: each is a UnicodeError and so a ValueError.
print("hier", issubclass(UnicodeDecodeError, UnicodeError),
      issubclass(UnicodeEncodeError, ValueError),
      issubclass(UnicodeTranslateError, UnicodeError))

# repr echoes the full argument tuple.
print("d_repr", repr(d))
print("e_repr", repr(e))
