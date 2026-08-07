# The integer string conversion limit (sys.int_max_str_digits, default 4300)
# is inclusive: a value with exactly 4300 decimal digits still renders, and
# only 4301+ raises. The largest 4300-digit int is 4300 nines, whose bit
# length sits one above the smallest 4300-digit int, so a bit-length fast
# path must not reject it.
def report(label, fn):
    try:
        print(label, "ok", fn())
    except ValueError as ex:
        print(label, "err", str(ex))

n = int("9" * 4300)
report("digits-at-limit", lambda: len(str(n)))
report("neg-at-limit", lambda: len(str(-n)))
report("format-d", lambda: len(format(n, "d")))
report("percent-d", lambda: len("%d" % n))
report("fstring", lambda: len(f"{n}"))
report("repr", lambda: len(repr(n)))

over = int("9" * 4300) * 10 + 9  # 4301 digits
report("over-limit-str", lambda: len(str(over)))
report("over-limit-format", lambda: format(over, "d"))

# int() parsing at and past the boundary
report("parse-4300", lambda: len(str(int("1" + "0" * 4299))))
report("parse-4301", lambda: int("1" * 4301))

# power-of-two bases stay exempt in either direction
report("hex-parse", lambda: int("f" * 5000, 16) > 0)
report("bin-of-big", lambda: len(bin(int("9" * 4300))) > 4300)
report("hex-of-big", lambda: len(hex(int("9" * 4300))) > 4300)
