# _tokenize.TokenizerIter is the C accelerator behind the pure-Python tokenize
# module: it drives the lexer and yields (type, string, start, end, line)
# 5-tuples that Lib/tokenize.py reshapes into TokenInfo records. This exercises
# the ported accelerator directly against the same source CPython tokenizes, so
# a token-numbering or position drift shows up before tokenize.py is involved.
import _tokenize
import io


def rows(src, extra):
    rl = io.StringIO(src).readline
    out = []
    for info in _tokenize.TokenizerIter(rl, extra_tokens=extra):
        typ, string, start, end, line = info
        out.append((typ, string, start, end))
    return out


src = "x = 1 + len('ab')\nif x:\n    y = x\n"
for r in rows(src, False):
    print(r)
print("--- extra ---")
for r in rows(src, True):
    print(r)

src2 = "def f(a, b):\n    return a * b\n"
print("--- def ---")
for r in rows(src2, False):
    print(r)

# A source with no trailing newline exercises the implicit-newline arm.
print("--- no-nl ---")
for r in rows("a+b", True):
    print(r)
