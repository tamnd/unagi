import codecs

# rot_13 is a str-to-str codec, so it is reached through codecs.encode/decode
# with a str on both sides rather than str.encode.
print("rot_enc", codecs.encode("Hello, World! 123", "rot_13"))
print("rot_dec", codecs.decode("Uryyb, Jbeyq! 123", "rot_13"))
print("rot_rt", codecs.decode(codecs.encode("Attack at dawn", "rot_13"), "rot_13"))

# punycode round-trips a range of scripts and symbols.
for s in ["munchen", "münchen", "bücher", "☃-⌘", "日本語", "αβγ"]:
    enc = codecs.encode(s, "punycode")
    print("puny", enc, codecs.decode(enc, "punycode") == s)
print("puny_method", "münchen".encode("punycode"))
print("puny_decmethod", b"mnchen-3ya".decode("punycode"))

# idna encodes each label to its ACE form and decodes it back.
for host in ["münchen.de", "bücher.example.com", "example.org", "aaa.bbb.ccc"]:
    enc = host.encode("idna")
    print("idna", enc, enc.decode("idna"))
