package objects

import (
	"encoding/hex"
	"math"
	"math/big"
	"testing"
)

// bigFromString builds a big.Int from a base-10 string for the wide-int vectors.
func bigFromString(t *testing.T, s string) *big.Int {
	t.Helper()
	n, ok := new(big.Int).SetString(s, 10)
	if !ok {
		t.Fatalf("bad big int literal %q", s)
	}
	return n
}

// TestPickleDumpsProtocol5 pins the pickler to CPython 3.14.6's protocol-5
// output byte for byte. Each want string is the hex of pickle.dumps(value, 5)
// captured from the reference interpreter, so any drift in opcode selection,
// integer width, framing, or memo discipline fails here.
func TestPickleDumpsProtocol5(t *testing.T) {
	cases := []struct {
		name string
		obj  Object
		want string
	}{
		{"none", None, "80054e2e"},
		{"true", True, "8005882e"},
		{"false", False, "8005892e"},
		{"zero", NewInt(0), "80054b002e"},
		{"one", NewInt(1), "80054b012e"},
		{"u8max", NewInt(255), "80054bff2e"},
		{"u16lo", NewInt(256), "80059504000000000000004d00012e"},
		{"u16max", NewInt(65535), "80059504000000000000004dffff2e"},
		{"u16over", NewInt(65536), "80059506000000000000004a000001002e"},
		{"neg1", NewInt(-1), "80059506000000000000004affffffff2e"},
		{"neg256", NewInt(-256), "80059506000000000000004a00ffffff2e"},
		{"i32max", NewInt(math.MaxInt32), "80059506000000000000004affffff7f2e"},
		{"i32over", NewInt(math.MaxInt32 + 1), "80059508000000000000008a0500000080002e"},
		{"u63", NewIntFromBig(bigFromString(t, "9223372036854775808")), "8005950c000000000000008a090000000000000080002e"},
		{"i64min", NewIntFromBig(bigFromString(t, "-9223372036854775808")), "8005950b000000000000008a0800000000000000802e"},
		{"i64max", NewIntFromBig(bigFromString(t, "9223372036854775807")), "8005950b000000000000008a08ffffffffffffff7f2e"},
		{"pi", NewFloat(3.14), "8005950a000000000000004740091eb851eb851f2e"},
		{"zerof", NewFloat(0.0), "8005950a000000000000004700000000000000002e"},
		{"negzerof", NewFloat(math.Copysign(0, -1)), "8005950a000000000000004780000000000000002e"},
		{"str_hi", NewStr("hi"), "80059506000000000000008c026869942e"},
		{"str_empty", NewStr(""), "80059504000000000000008c00942e"},
		{"str_utf8", NewStr("héllo"), "8005950a000000000000008c0668c3a96c6c6f942e"},
		{"bytes_ab", NewBytes([]byte("ab")), "800595060000000000000043026162942e"},
		{"bytes_empty", NewBytes([]byte{}), "80059504000000000000004300942e"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := PickleDumps(tc.obj, 5)
			if err != nil {
				t.Fatalf("PickleDumps(%s): %v", tc.name, err)
			}
			if h := hex.EncodeToString(got); h != tc.want {
				t.Fatalf("PickleDumps(%s)\n got  %s\n want %s", tc.name, h, tc.want)
			}
		})
	}
}

// TestPickleRoundTrip confirms the loader reconstructs every scalar the pickler
// emits, so dumps followed by loads is an identity on the value.
func TestPickleRoundTrip(t *testing.T) {
	values := []Object{
		None, True, False,
		NewInt(0), NewInt(1), NewInt(255), NewInt(256), NewInt(65535), NewInt(65536),
		NewInt(-1), NewInt(-256), NewInt(math.MaxInt32), NewInt(math.MaxInt32 + 1),
		NewIntFromBig(bigFromString(t, "9223372036854775808")),
		NewIntFromBig(bigFromString(t, "-9223372036854775808")),
		NewIntFromBig(bigFromString(t, "170141183460469231731687303715884105727")),
		NewFloat(3.14), NewFloat(0.0), NewFloat(-2.5e300),
		NewStr(""), NewStr("hi"), NewStr("héllo"),
		NewBytes([]byte{}), NewBytes([]byte("ab")), NewBytes([]byte{0, 1, 2, 255}),
	}
	for _, proto := range []int{2, 3, 4, 5} {
		for _, v := range values {
			// CPython encodes bytes at protocol 2 through a codecs.encode
			// reduction (SHORT_BINBYTES is protocol 3+); that reduction machinery
			// arrives with the object protocol, so bytes are exercised at 3+ here.
			if _, isBytes := v.(*bytesObject); isBytes && proto < 3 {
				continue
			}
			data, err := PickleDumps(v, proto)
			if err != nil {
				t.Fatalf("dumps(proto=%d) %s: %v", proto, Repr(v), err)
			}
			back, err := PickleLoads(data)
			if err != nil {
				t.Fatalf("loads(proto=%d) %s: %v", proto, Repr(v), err)
			}
			if !pickleScalarEqual(v, back) {
				t.Fatalf("roundtrip(proto=%d) mismatch: %s -> %s", proto, Repr(v), Repr(back))
			}
		}
	}
}

// pickleScalarEqual compares two scalar objects by value for the round-trip
// check, matching on type and contents the way Python == would for these leaves.
func pickleScalarEqual(a, b Object) bool {
	switch av := a.(type) {
	case *noneObject:
		_, ok := b.(*noneObject)
		return ok
	case *boolObject:
		bv, ok := b.(*boolObject)
		return ok && av.v == bv.v
	case *intObject:
		x, ok := AsBigInt(a)
		y, ok2 := AsBigInt(b)
		return ok && ok2 && x.Cmp(y) == 0
	case *floatObject:
		y, ok := AsFloat(b)
		return ok && (av.v == y || (math.IsNaN(av.v) && math.IsNaN(y)))
	case *strObject:
		y, ok := AsStr(b)
		return ok && av.v == y
	case *bytesObject:
		y, ok := AsBytes(b)
		if !ok || len(av.v) != len(y) {
			return false
		}
		for i := range av.v {
			if av.v[i] != y[i] {
				return false
			}
		}
		return true
	}
	return false
}

// TestPickleProtocolDifferences pins the cross-protocol opcode selection that
// framing and the short-opcode tiers introduce, so a lower protocol keeps its
// distinct (also CPython-captured) bytes rather than silently emitting proto-5.
// Each want is the hex of pickle.dumps(value, proto) from CPython 3.14.6.
func TestPickleProtocolDifferences(t *testing.T) {
	cases := []struct {
		name  string
		obj   Object
		proto int
		want  string
	}{
		// str: BINUNICODE + explicit BINPUT memo at 2/3 (no framing);
		// SHORT_BINUNICODE + MEMOIZE inside a FRAME at 4+.
		{"str_hi_p2", NewStr("hi"), 2, "80025802000000686971002e"},
		{"str_hi_p3", NewStr("hi"), 3, "80035802000000686971002e"},
		{"str_hi_p4", NewStr("hi"), 4, "80049506000000000000008c026869942e"},
		// bytes: SHORT_BINBYTES + explicit BINPUT at 3 (no framing).
		{"bytes_ab_p3", NewBytes([]byte("ab")), 3, "80034302616271002e"},
		// int: BININT2 is protocol-agnostic and never memoized.
		{"int256_p2", NewInt(256), 2, "80024d00012e"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := PickleDumps(tc.obj, tc.proto)
			if err != nil {
				t.Fatalf("PickleDumps(%s): %v", tc.name, err)
			}
			if h := hex.EncodeToString(got); h != tc.want {
				t.Fatalf("PickleDumps(%s)\n got  %s\n want %s", tc.name, h, tc.want)
			}
		})
	}
}
