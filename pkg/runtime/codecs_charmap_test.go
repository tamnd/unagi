package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// charmapCall drives one of the charmap kw-functions and returns the (result, len)
// tuple elements.
func charmapCall(t *testing.T, fn func([]objects.Object, []string, []objects.Object) (objects.Object, error), args ...objects.Object) (objects.Object, int64) {
	t.Helper()
	res, err := fn(args, nil, nil)
	if err != nil {
		t.Fatalf("charmap call: %v", err)
	}
	out, err := objects.GetItem(res, objects.NewInt(0))
	if err != nil {
		t.Fatalf("tuple[0]: %v", err)
	}
	n, err := objects.GetItem(res, objects.NewInt(1))
	if err != nil {
		t.Fatalf("tuple[1]: %v", err)
	}
	iv, _ := objects.AsInt(n)
	return out, iv
}

func TestCharmapBuildAndRoundtrip(t *testing.T) {
	// An identity table roundtrips every byte and reports the input length.
	var sb []rune
	for i := 0; i < 256; i++ {
		sb = append(sb, rune(i))
	}
	table := objects.NewStr(string(sb))
	enc, err := codecCharmapBuild([]objects.Object{table})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	out, n := charmapCall(t, codecCharmapEncode, objects.NewStr("AZ"), objects.NewStr("strict"), enc)
	if b, _ := objects.AsBytesLike(out); string(b) != "AZ" || n != 2 {
		t.Fatalf("encode identity: %q n=%d", b, n)
	}
	dout, dn := charmapCall(t, codecCharmapDecode, objects.NewBytes([]byte("AZ")), objects.NewStr("strict"), table)
	if s, _ := objects.AsStr(dout); s != "AZ" || dn != 2 {
		t.Fatalf("decode identity: %q n=%d", s, dn)
	}
}

func TestCharmapBuildLastWins(t *testing.T) {
	// A code point at two byte positions builds to the higher byte, and the U+FFFE
	// sentinel is kept rather than skipped, matching CPython's EncodingMap build.
	runes := []rune{'A', 'A'}
	for i := 2; i < 256; i++ {
		runes = append(runes, rune(i))
	}
	enc, err := codecCharmapBuild([]objects.Object{objects.NewStr(string(runes))})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	// 'A' also sits at byte 0x41 in the tail, so the highest byte wins.
	out, _ := charmapCall(t, codecCharmapEncode, objects.NewStr("A"), objects.NewStr("strict"), enc)
	if b, _ := objects.AsBytesLike(out); len(b) != 1 || b[0] != 0x41 {
		t.Fatalf("last wins: %x", b)
	}
}

func TestCharmapDecodeErrors(t *testing.T) {
	runes := []rune{0xFFFE}
	for i := 1; i < 256; i++ {
		runes = append(runes, rune(i))
	}
	table := objects.NewStr(string(runes))
	// strict raises the charmap error with CPython's wording.
	if _, _, err := func() (objects.Object, int64, error) {
		res, err := codecCharmapDecode([]objects.Object{objects.NewBytes([]byte{0x00}), objects.NewStr("strict"), table}, nil, nil)
		return res, 0, err
	}(); err == nil {
		t.Fatalf("strict: expected error")
	}
	// ignore drops the byte, replace emits U+FFFD, both keeping the input length.
	out, n := charmapCall(t, codecCharmapDecode, objects.NewBytes([]byte{0x00, 'A'}), objects.NewStr("ignore"), table)
	if s, _ := objects.AsStr(out); s != "A" || n != 2 {
		t.Fatalf("ignore: %q n=%d", s, n)
	}
	out, _ = charmapCall(t, codecCharmapDecode, objects.NewBytes([]byte{0x00, 'A'}), objects.NewStr("replace"), table)
	if s, _ := objects.AsStr(out); s != "�A" {
		t.Fatalf("replace: %q", s)
	}
}

func TestCharmapEncodeValueTypes(t *testing.T) {
	// A dict mapping accepts int and bytes values and rejects an out-of-range int.
	m, err := objects.NewDict(
		[]objects.Object{objects.NewInt('A'), objects.NewInt('B')},
		[]objects.Object{objects.NewInt(0x61), objects.NewBytes([]byte("xy"))},
	)
	if err != nil {
		t.Fatalf("dict: %v", err)
	}
	out, n := charmapCall(t, codecCharmapEncode, objects.NewStr("AB"), objects.NewStr("strict"), m)
	if b, _ := objects.AsBytesLike(out); string(b) != "axy" || n != 2 {
		t.Fatalf("dict encode: %q n=%d", b, n)
	}
	bad, _ := objects.NewDict([]objects.Object{objects.NewInt('A')}, []objects.Object{objects.NewInt(300)})
	if _, err := codecCharmapEncode([]objects.Object{objects.NewStr("A"), objects.NewStr("strict"), bad}, nil, nil); err == nil {
		t.Fatalf("range(256): expected TypeError")
	}
}
