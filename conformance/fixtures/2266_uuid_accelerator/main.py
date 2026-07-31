"""The _uuid C helper backs uuid.py's _generate_time_safe, so uuid1() with no
node/clock_seq gets a version-1 UUID from the platform generator. The concrete
UUID contents -- the node, the safety signal, the capability flags -- vary by
platform (libuuid deduces a real MAC on some builds and not others), so this
checks only what is invariant everywhere: the generator's output shape and that
uuid1() yields proper, unique version-1 UUIDs, plus the deterministic
node/clock_seq-specified and name-based paths."""

import _uuid
import uuid

# generate_time_safe() -> (16 bytes, <safety>). The bytes always decode to a
# version-1, RFC 4122 UUID regardless of platform; the safety signal itself is
# platform-dependent, so only the byte shape is checked.
raw, _ = _uuid.generate_time_safe()
print("generate_time_safe bytes len :", len(raw))
u = uuid.UUID(bytes=raw)
print("decoded version / variant    :", u.version, u.variant)

# uuid1() with no arguments routes through _uuid; every result is a proper,
# distinct version-1 UUID.
batch = [uuid.uuid1() for _ in range(500)]
print("all version 1                :", all(u.version == 1 for u in batch))
print("all RFC 4122                 :", all(u.variant == uuid.RFC_4122 for u in batch))
print("all unique                   :", len(set(batch)) == 500)

# A supplied node and clock sequence bypass _uuid and appear verbatim -- fully
# deterministic across platforms.
u = uuid.uuid1(0x123456789ABC, 0x1234)
print("supplied node                :", format(u.node, "012x"))
print("supplied clock_seq           :", ((u.clock_seq_hi_variant & 0x3F) << 8) | u.clock_seq_low)
print("supplied version / variant   :", u.version, u.variant)

# uuid3/uuid5 (name-based, pure Python, unaffected) still work alongside.
print("uuid5 dns python.org         :", uuid.uuid5(uuid.NAMESPACE_DNS, "python.org"))
print("uuid3 dns python.org         :", uuid.uuid3(uuid.NAMESPACE_DNS, "python.org"))
