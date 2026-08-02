# str.isalpha, str.isalnum, str.isdecimal, str.isdigit, str.isnumeric and
# str.isprintable classify each character with the general category and the
# digit/numeric value tables. They read from the pinned CPython 3.14.6 UCD, which
# covers the newer blocks Go's older unicode data misses: the Garay letters count
# as alphabetic and the new capital-letter additions classify the way CPython
# does.
print("alpha plain", "Hello".isalpha())
print("alpha digit", "abc1".isalpha())
print("alnum plain", "abc123".isalnum())
print("alnum punct", "abc!".isalnum())
print("empty", "".isalpha(), "".isalnum(), "".isdecimal(), "".isprintable())

# The Garay block (Unicode 16.0.0) is a set of letters, so its code points are
# alphabetic and alphanumeric even though older tables do not know them.
print("garay alpha", "\U00010d50".isalpha(), "\U00010d70".isalpha())
print("garay alnum", "\U00010d50\U00010d70".isalnum())

# The three numeric predicates split by value: a decimal digit is decimal, digit
# and numeric; a superscript is a digit and numeric but not decimal; a Roman
# numeral and a circled number are numeric only; a fraction is numeric only.
for s in ["7", "²", "Ⅶ", "⑦", "½", "๓", "〇"]:
    print("num", repr(s), s.isdecimal(), s.isdigit(), s.isnumeric(), s.isalnum())

# A letter is none of the numeric classes, and a CJK ideograph with a numeric
# value counts as numeric but not as a digit.
print("letter num", "A".isdecimal(), "A".isdigit(), "A".isnumeric())
print("cjk num", "四".isnumeric(), "四".isdigit(), "四".isalpha())

# isprintable keeps letters, digits, punctuation and symbols but drops the
# control, format and separator characters, with the ASCII space the one
# separator kept. The empty string is printable.
for s in ["Hello, World!", "abc def", "\x1b", "\x7f", "​", " ",
          "\U0001f600", "line\nbreak", " "]:
    print("printable", repr(s), s.isprintable())

# The predicates take no arguments and are bound methods off the instance.
try:
    "x".isalpha(1)
except TypeError as e:
    print("arity", e)
print("callable", callable("x".isnumeric), callable("x".isprintable))

# A lone surrogate is category Cs: not alphabetic, not numeric and not printable,
# on its own and mixed into text.
low = b"\xed\xb2\x80".decode("utf-8", "surrogatepass")
print("surrogate", low.isalpha(), low.isnumeric(), low.isprintable())
print("surrogate mixed", ("A" + low).isalpha(), ("A" + low).isprintable())
