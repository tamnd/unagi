# The standard non-strict codec error handlers, exercised through real encode
# and decode calls: str.encode over the ascii/latin-1/utf-8 codecs, bytes.decode
# for the decode side, and codecs.encode over a multibyte codec so the same
# handlers run on the CJK path.
import codecs

s = "abcሴdef\U0001F600ghi"

for name in ("ignore", "replace", "xmlcharrefreplace", "backslashreplace"):
    print(name, s.encode("ascii", name))

# latin-1 keeps the sub-0x100 run, so only the two high code points hit the
# handler.
print("latin-1 xml", s.encode("latin-1", "xmlcharrefreplace"))
print("latin-1 bs", s.encode("latin-1", "backslashreplace"))

# Decode side: the bad bytes are escaped or dropped, and decoding resumes past
# them.
bad = b"a\xff\xfeb"
for name in ("ignore", "replace", "backslashreplace"):
    print("decode", name, bad.decode("ascii", name))

# The multibyte codec path runs the registered handlers through codecs.encode.
jp = "・一xyz￿"
for name in ("ignore", "replace", "xmlcharrefreplace", "backslashreplace"):
    print("euc_jp", name, codecs.encode(jp, "euc_jp", name))

# codecs.lookup_error returns the registered handler object for each name.
for name in ("ignore", "replace", "xmlcharrefreplace", "backslashreplace"):
    print("lookup", name, callable(codecs.lookup_error(name)))
