# The \N{NAME} escape names a character from the Unicode database, resolving at
# compile time to the same code point unicodedata.lookup would return.
print("nbsp:", ord("\N{NO-BREAK SPACE}"))
print("narrow:", ord("\N{NARROW NO-BREAK SPACE}"))
print("bullet:", ord("\N{BULLET}"))
print("arrow:", ord("\N{RIGHTWARDS ARROW}"))

# A named sequence and an alias resolve too.
print("null alias:", ord("\N{NULL}"))
print("latin a:", "\N{LATIN SMALL LETTER A}")

# An algorithmic Hangul syllable name resolves by the same rule.
print("hangul:", "\N{HANGUL SYLLABLE GA}")

# The escape works joined with other text and inside an f-string literal part.
print("joined:", "a\N{BULLET}b")
tag = "x"
print("fstring:", f"[{tag}]\N{BULLET}")

# It agrees with unicodedata.lookup for the same name.
import unicodedata
print("agrees lookup:", "\N{SNOWMAN}" == unicodedata.lookup("SNOWMAN"))


def raises(fn, *a):
    try:
        fn(*a)
    except SyntaxError:
        return True
    except BaseException:
        return False
    return False


# A malformed escape and an unknown name are SyntaxErrors at compile time, which
# eval surfaces for a string built without the escape being pre-parsed.
print("unknown name raises:", raises(eval, r"'\N{NOT A REAL NAME}'"))
print("malformed raises:", raises(eval, r"'\N'"))
