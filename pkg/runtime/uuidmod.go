package runtime

import (
	"crypto/rand"
	"time"

	"github.com/tamnd/unagi/pkg/objects"
)

// _uuid is the small C helper behind the public uuid module. uuid.py does
// `import _uuid` and, when it succeeds, binds _uuid.generate_time_safe as its
// _generate_time_safe so uuid1() with no node/clock_seq gets a version-1 UUID
// from the platform generator instead of the pure-Python fallback. It also reads
// two capability flags. uuid.py degrades gracefully when the import fails, so
// `import uuid` already worked; this makes `import _uuid` itself importable and
// lets test_uuid's TestUUIDWithExtModule (skipUnless the C module) run the whole
// UUID suite against the accelerated path.
//
// CPython exposes generate_time_safe, has_stable_extractable_node and
// has_uuid_generate_time_safe. On a build whose libuuid lacks the "safe" variant
// and cannot deduce a stable MAC (the pinned macOS oracle) both flags read 0 and
// generate_time_safe returns (uuid_bytes, None); the libuuid-specific node tests
// skip on that config. This shim reproduces that shape: a genuine version-1 UUID
// with a random 48-bit node and 14-bit clock sequence -- which is what
// uuid1()'s no-argument path needs to be version 1, RFC 4122, and unique across
// many calls -- paired with an unknown ("None") safety, and both flags 0.

func init() {
	moduleTable["_uuid"] = &moduleEntry{builtin: true, exec: initUUID}
}

func initUUID(m *objects.Module) error {
	if err := objects.StoreAttr(m, "generate_time_safe",
		objects.NewFunc("generate_time_safe", 0, uuidGenerateTimeSafe)); err != nil {
		return err
	}
	// Both capabilities read 0 on the pinned build: the libuuid here has neither
	// the thread-safe generator nor a stably extractable node, so uuid.py takes
	// the unknown-safety path and skips the libuuid node getters.
	if err := objects.StoreAttr(m, "has_stable_extractable_node", objects.NewInt(0)); err != nil {
		return err
	}
	return objects.StoreAttr(m, "has_uuid_generate_time_safe", objects.NewInt(0))
}

// uuidGenerateTimeSafe is _uuid.generate_time_safe(). It returns a 2-tuple of the
// 16 raw bytes of a version-1 UUID and the "safely generated" flag. This build's
// generator has no safety signal, so the flag is None, which uuid.py maps to
// SafeUUID.unknown -- matching the pinned oracle, where the safe variant is
// absent.
func uuidGenerateTimeSafe(args []objects.Object) (objects.Object, error) {
	b, err := uuidV1Bytes()
	if err != nil {
		return nil, err
	}
	return objects.NewTuple([]objects.Object{objects.NewBytes(b), objects.None}), nil
}

// uuidV1Bytes builds the 16 big-endian bytes of a version-1 UUID: a 60-bit
// timestamp of 100-ns intervals since the 1582-10-15 Gregorian epoch, a random
// 14-bit clock sequence, and a random 48-bit node with the multicast bit set
// (the convention uuid.getnode() uses when it cannot read a hardware address).
// The version nibble and the RFC 4122 variant bits are stamped in place. The
// random clock sequence and node give every call distinct bytes even when two
// land in the same 100-ns tick, so a burst of uuid1() calls stays unique.
func uuidV1Bytes() ([]byte, error) {
	// 0x01b21dd213814000 is the count of 100-ns intervals between the UUID epoch
	// (1582-10-15) and the Unix epoch (1970-01-01), the same constant uuid.py uses.
	const gregorianOffset = 0x01b21dd213814000
	timestamp := uint64(time.Now().UnixNano()/100) + gregorianOffset

	// 8 random bytes: 2 seed the clock sequence, 6 the node.
	var r [8]byte
	if _, err := rand.Read(r[:]); err != nil {
		return nil, objects.Raise("OSError", "uuid: %s", err.Error())
	}

	timeLow := uint32(timestamp & 0xffffffff)
	timeMid := uint16((timestamp >> 32) & 0xffff)
	timeHiVersion := uint16((timestamp>>48)&0x0fff) | (1 << 12) // version 1
	clockSeq := (uint16(r[0])<<8 | uint16(r[1])) & 0x3fff

	b := make([]byte, 16)
	b[0] = byte(timeLow >> 24)
	b[1] = byte(timeLow >> 16)
	b[2] = byte(timeLow >> 8)
	b[3] = byte(timeLow)
	b[4] = byte(timeMid >> 8)
	b[5] = byte(timeMid)
	b[6] = byte(timeHiVersion >> 8)
	b[7] = byte(timeHiVersion)
	b[8] = byte(clockSeq>>8) | 0x80 // RFC 4122 variant (top bits 10)
	b[9] = byte(clockSeq)
	copy(b[10:], r[2:8])
	b[10] |= 0x01 // multicast bit: a locally administered, non-hardware node
	return b, nil
}
