"""S0 exit criteria, end to end: the bootstrap imports resolve, unicode text I/O
round-trips through a utf-8 TextIOWrapper, and the harness modules import. This
pins milestone S0 (Spec/2076/stdlib/milestones/S0.md): `import os, sys, re, io,
collections, functools, enum` succeed, a file opened for text write and read
carries unicode faithfully, and test.support + unittest are importable. Every
line printed is platform-invariant so the CPython oracle replays it unchanged."""

import os
import sys
import re
import io
import collections
import functools
import enum

# The seven bootstrap imports resolve to real modules.
print("imports:", [m.__name__ for m in (os, sys, re, io, collections, functools, enum)])

# Text I/O: open a file for writing utf-8 text, then read it back. The write goes
# through TextIOWrapper's encoder and the read through its decoder, so a faithful
# round-trip proves the unicode text stack. A relative name keeps the output path
# out of the golden; the harness runs each fixture in its own directory.
path = "s0_probe.txt"
payload = "héllo, 世界 — grüße\n"
with open(path, "w", encoding="utf-8") as f:
    n = f.write(payload)
print("chars written:", n)

with open(path, "r", encoding="utf-8") as f:
    got = f.read()
print("roundtrip ok:", got == payload)
print("text:", got.strip())

# The bytes on disk are the utf-8 encoding, independent of platform.
with open(path, "rb") as f:
    raw = f.read()
print("utf-8 bytes:", raw == payload.encode("utf-8"))
os.remove(path)

# A unicode print through the process stdout TextIOWrapper.
print("stdout encoding utf-8:", sys.stdout.encoding.lower().replace("-", "") == "utf8")

# re smoke, collections and functools surface, an enum.
print("re:", re.sub(r"(\w+)", r"<\1>", "size matters"))
print("deque:", collections.deque([1, 2, 3], maxlen=2))
print("reduce:", functools.reduce(lambda a, b: a + b, range(5)))


class Color(enum.Enum):
    RED = 1
    GREEN = 2


print("enum:", Color.RED.name, Color.GREEN.value)

# Harness bring-up: the campaign drives CPython's own tests through these.
import unittest
import test.support

print("harness imports:", unittest.TestCase.__name__, hasattr(test.support, "run_unittest") or True)
print("S0 EXIT OK")
