"""The _uuid C helper backs uuid.py's _generate_time_safe, so uuid1() with no
node/clock_seq gets a version-1 UUID from the platform generator. On the pinned
build both capability flags read 0 and the generator has no safety signal, so
generate_time_safe returns (bytes, None). UUID contents are random, so this
checks the deterministic structure: the flags, the tuple shape, and that a batch
of uuid1() values are all version 1, RFC 4122, unknown-safety, and unique."""

import _uuid
import uuid

# The two capability flags are the integer 0 on this build.
print("has_stable_extractable_node   :", _uuid.has_stable_extractable_node)
print("has_uuid_generate_time_safe   :", _uuid.has_uuid_generate_time_safe)

# generate_time_safe() -> (16 bytes, None). The bytes decode to a version-1 UUID.
raw, safe = _uuid.generate_time_safe()
print("generate_time_safe bytes len  :", len(raw))
print("generate_time_safe safety flag:", safe)
u = uuid.UUID(bytes=raw)
print("decoded version / variant     :", u.version, u.variant)

# uuid1() with no arguments now routes through _uuid; every result is a proper,
# distinct version-1 UUID.
batch = [uuid.uuid1() for _ in range(500)]
print("all version 1                 :", all(u.version == 1 for u in batch))
print("all RFC 4122                  :", all(u.variant == uuid.RFC_4122 for u in batch))
print("all unknown-safety            :", all(u.is_safe is uuid.SafeUUID.unknown for u in batch))
print("all unique                    :", len(set(batch)) == 500)

# A supplied node and clock sequence bypass _uuid and appear verbatim.
u = uuid.uuid1(0x123456789abc, 0x1234)
print("supplied node                 :", format(u.node, "012x"))
print("supplied clock_seq            :", ((u.clock_seq_hi_variant & 0x3f) << 8) | u.clock_seq_low)
print("supplied version / variant    :", u.version, u.variant)

# uuid3/uuid5 (name-based, pure Python, unaffected) still work alongside.
print("uuid5 dns python.org          :", uuid.uuid5(uuid.NAMESPACE_DNS, "python.org"))
