# gettext covers three surfaces: NullTranslations fallbacks, the c2py plural
# expression compiler, and GNUTranslations parsing a real GNU .mo catalog.
# The catalog is built in memory and loaded through io.BytesIO so the fixture
# stays filesystem free and fully deterministic.
import gettext
import struct
import io

# NullTranslations returns its arguments unchanged.
n = gettext.NullTranslations()
print("null", repr(n.gettext("hello")), repr(n.ngettext("one", "many", 1)),
      repr(n.ngettext("one", "many", 5)))
print("null", repr(n.pgettext("ctx", "hi")), repr(n.npgettext("ctx", "one", "many", 2)))

# Module level lookups with no bound domain fall back to identity.
print("mod", repr(gettext.gettext("world")), repr(gettext.dgettext("nodomain", "x")))
print("mod", repr(gettext.ngettext("a", "b", 1)), repr(gettext.ngettext("a", "b", 3)))

# c2py compiles the C plural expression to a Python callable.
for expr, vals in [
    ("n != 1", [0, 1, 2]),
    ("n>1", [0, 1, 2, 5]),
    ("n%10==1 && n%100!=11 ? 0 : 1", [1, 11, 21, 2]),
    ("(n==0 ? 0 : n==1 ? 1 : 2)", [0, 1, 2]),
]:
    f = gettext.c2py(expr)
    print("c2py", repr(expr), [f(v) for v in vals])


def make_mo(entries, metadata):
    items = [(b"", metadata.encode())] + [(k.encode(), v.encode()) for k, v in entries]
    items.sort(key=lambda x: x[0])
    count = len(items)
    keys = b""
    vals = b""
    koff = []
    voff = []
    for k, v in items:
        koff.append((len(k), len(keys)))
        keys += k + b"\x00"
    for k, v in items:
        voff.append((len(v), len(vals)))
        vals += v + b"\x00"
    keystart = 7 * 4 + 16 * count
    valstart = keystart + len(keys)
    out = struct.pack("<Iiiiiii", 0x950412de, 0, count, 7 * 4, 7 * 4 + 8 * count, 0, 0)
    for ln, off in koff:
        out += struct.pack("<ii", ln, keystart + off)
    for ln, off in voff:
        out += struct.pack("<ii", ln, valstart + off)
    return out + keys + vals


meta = "Content-Type: text/plain; charset=UTF-8\nPlural-Forms: nplurals=2; plural=n != 1;\n"
data = make_mo([
    ("hello", "hola"),
    ("world", "mundo"),
    ("apple\x00apples", "manzana\x00manzanas"),
    ("ctx\x04menu", "menú"),
], meta)

t = gettext.GNUTranslations(io.BytesIO(data))
print("gnu", repr(t.gettext("hello")), repr(t.gettext("world")), repr(t.gettext("missing")))
print("gnu", repr(t.ngettext("apple", "apples", 1)), repr(t.ngettext("apple", "apples", 5)))
print("gnu", repr(t.pgettext("ctx", "menu")), repr(t.pgettext("ctx", "absent")))
print("gnu", repr(t.info()["content-type"]))

# The bound method can be aliased, the common `_ = t.gettext` idiom.
underscore = t.gettext
print("alias", repr(underscore("hello")), repr(underscore("missing")))
