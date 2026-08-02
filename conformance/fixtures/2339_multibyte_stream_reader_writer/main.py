# _multibytecodec's MultibyteStreamReader and MultibyteStreamWriter wrap an
# underlying byte stream and carry the same incremental decoder/encoder state
# across calls: the reader pulls bytes and decodes holding an incomplete tail, the
# writer encodes holding a combining base or shift state. read, readline and
# readlines all decode through the same path, and writing a string one character at
# a time still produces the whole encoding. Instantiating the bare classes with no
# codec raises AttributeError (CPython bug #3305, which used to segfault). This
# pins unagi to CPython across a per-unit codec (euc_kr) and a shift-state one
# (iso2022_jp).
import codecs
from io import BytesIO
import _multibytecodec as mbc

samples = {
    "euc_kr": "한글ABC가나다쓔글\n둘째 줄\n끝",
    "iso2022_jp": "abcあいう\ndef日本語\n",
}

for enc, text in samples.items():
    ci = codecs.lookup(enc)
    raw = text.encode(enc)

    # Writing one character at a time keeps encoder state across write() calls.
    w = ci.streamwriter(BytesIO())
    for ch in text:
        w.write(ch)
    print(enc, "write-by-char", w.stream.getvalue() == raw)

    # writelines takes a sequence and writes each item.
    w = ci.streamwriter(BytesIO())
    w.writelines(text.splitlines(True))
    print(enc, "writelines", w.stream.getvalue() == raw)

    # read, readline and readlines all reconstruct the text across sizehints.
    for name in ["read", "readline", "readlines"]:
        ok = True
        for sizehint in [None, -1, 1, 2, 3, 5, 7, 64]:
            r = ci.streamreader(BytesIO(raw))
            func = getattr(r, name)
            acc = []
            while True:
                data = func(sizehint)
                if not data:
                    break
                if name == "readlines":
                    acc.extend(data)
                else:
                    acc.append(data)
            if "".join(acc) != text:
                ok = False
        print(enc, name, ok)

    # reset drops reader state and flushes writer state.
    r = ci.streamreader(BytesIO(raw))
    r.read(3)
    print(enc, "reader-reset", r.reset())
    w = ci.streamwriter(BytesIO())
    w.write(text)
    print(enc, "writer-reset", w.reset())

# The bare classes raise AttributeError when built with no codec attribute.
for cls in (mbc.MultibyteStreamReader, mbc.MultibyteStreamWriter):
    try:
        cls(None)
        print(cls.__name__, "no-raise")
    except AttributeError:
        print(cls.__name__, "AttributeError")
