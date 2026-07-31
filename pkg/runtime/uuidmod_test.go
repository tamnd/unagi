package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// TestUUIDModuleFlags checks _uuid imports and reports both capability flags as
// the integer 0, matching the pinned oracle whose libuuid has neither the safe
// generator nor a stably extractable node.
func TestUUIDModuleFlags(t *testing.T) {
	mo, err := ImportModule("_uuid")
	if err != nil {
		t.Fatalf("import _uuid: %v", err)
	}
	for _, name := range []string{"has_stable_extractable_node", "has_uuid_generate_time_safe"} {
		v, err := objects.LoadAttr(mo, name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if n, ok := objects.AsInt(v); !ok || n != 0 {
			t.Errorf("%s = %v, want 0", name, v)
		}
	}
}

// TestUUIDGenerateTimeSafe checks generate_time_safe returns a (bytes, None)
// pair whose bytes form a well-formed version-1 UUID -- version nibble 1, the
// RFC 4122 variant bits, and the multicast node bit -- and that repeated calls
// never collide.
func TestUUIDGenerateTimeSafe(t *testing.T) {
	mo, err := ImportModule("_uuid")
	if err != nil {
		t.Fatalf("import _uuid: %v", err)
	}
	fn, err := objects.LoadAttr(mo, "generate_time_safe")
	if err != nil {
		t.Fatalf("generate_time_safe: %v", err)
	}
	seen := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		r, err := objects.Call(fn, nil)
		if err != nil {
			t.Fatalf("call: %v", err)
		}
		tup, err := objects.Iter(r)
		if err != nil {
			t.Fatalf("iter result: %v", err)
		}
		first, _, _ := tup.Next()
		second, _, _ := tup.Next()
		if second != objects.None {
			t.Fatalf("safety flag = %v, want None", second)
		}
		b, ok := objects.AsBufferBytes(first)
		if !ok || len(b) != 16 {
			t.Fatalf("bytes = %v (len ok=%v), want 16 bytes", first, ok)
		}
		// Version 1: high nibble of byte 6.
		if v := b[6] >> 4; v != 1 {
			t.Errorf("version nibble = %d, want 1", v)
		}
		// RFC 4122 variant: top two bits of byte 8 are 10.
		if b[8]&0xc0 != 0x80 {
			t.Errorf("variant bits = %#x, want 0x80", b[8]&0xc0)
		}
		// Locally administered node: multicast bit of byte 10 is set.
		if b[10]&0x01 != 0x01 {
			t.Errorf("multicast bit unset in node byte %#x", b[10])
		}
		key := string(b)
		if seen[key] {
			t.Fatalf("duplicate UUID bytes at call %d", i)
		}
		seen[key] = true
	}
}
