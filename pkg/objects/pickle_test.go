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

// TestPickleDumpsContainersProtocol5 pins the container wire format against
// CPython 3.14.6: the tuple opcode tiers (EMPTY_TUPLE, TUPLE1/2/3, MARK+TUPLE),
// list EMPTY_LIST + APPEND/APPENDS batching, dict EMPTY_DICT + SETITEM/SETITEMS,
// nesting, and the memo GET that a shared child produces.
func TestPickleDumpsContainersProtocol5(t *testing.T) {
	// shared child list, referenced twice.
	shared := &listObject{elts: []Object{NewInt(1)}}
	cases := []struct {
		name string
		obj  Object
		want string
	}{
		{"empty_tuple", NewTuple(nil), "8005292e"},
		{"tuple1", NewTuple([]Object{NewInt(1)}), "80059505000000000000004b0185942e"},
		{"tuple2", NewTuple([]Object{NewInt(1), NewInt(2)}), "80059507000000000000004b014b0286942e"},
		{"tuple3", NewTuple([]Object{NewInt(1), NewInt(2), NewInt(3)}), "80059509000000000000004b014b024b0387942e"},
		{"tuple4", NewTuple([]Object{NewInt(1), NewInt(2), NewInt(3), NewInt(4)}), "8005950c00000000000000284b014b024b034b0474942e"},
		{"nested_tuple", NewTuple([]Object{NewInt(1), NewTuple([]Object{NewInt(2), NewInt(3)})}), "8005950b000000000000004b014b024b03869486942e"},
		{"empty_list", NewList(nil), "80055d942e"},
		{"list1", NewList([]Object{NewInt(1)}), "80059506000000000000005d944b01612e"},
		{"list123", NewList([]Object{NewInt(1), NewInt(2), NewInt(3)}), "8005950b000000000000005d94284b014b024b03652e"},
		{"nested_list", NewList([]Object{NewInt(1), NewList([]Object{NewInt(2), NewInt(3)}), NewStr("x")}), "80059513000000000000005d94284b015d94284b024b03658c017894652e"},
		{"empty_dict", mustDict(), "80057d942e"},
		{"dict1", mustDict(NewStr("k"), NewInt(9)), "8005950a000000000000007d948c016b944b09732e"},
		{"dict_ab", mustDict(NewStr("a"), NewInt(1), NewStr("b"), NewInt(2)), "80059511000000000000007d94288c0161944b018c0162944b02752e"},
		{"shared_list", NewList([]Object{shared, shared}), "8005950c000000000000005d94285d944b01616801652e"},
		{"tuple_shared", NewTuple([]Object{shared, shared}), "8005950a000000000000005d944b0161680086942e"},
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

// TestPickleContainerRoundTrip dumps then loads a spread of nested and shared
// containers and checks the result reprs identically, so the loader rebuilds the
// structure the pickler wrote across every binary protocol.
func TestPickleContainerRoundTrip(t *testing.T) {
	shared := &listObject{elts: []Object{NewInt(1), NewStr("s")}}
	values := []Object{
		NewTuple(nil),
		NewTuple([]Object{NewInt(1), NewStr("two"), NewFloat(3.0)}),
		NewTuple([]Object{NewInt(1), NewInt(2), NewInt(3), NewInt(4), NewInt(5)}),
		NewList(nil),
		NewList([]Object{NewInt(1), NewList([]Object{NewInt(2), NewInt(3)}), NewStr("x")}),
		mustDict(NewStr("a"), NewInt(1), NewStr("b"), NewList([]Object{NewInt(2)})),
		NewList([]Object{shared, shared}),
		NewTuple([]Object{NewStr("k"), mustDict(NewInt(1), NewStr("v"))}),
	}
	for _, proto := range []int{2, 3, 4, 5} {
		for _, v := range values {
			data, err := PickleDumps(v, proto)
			if err != nil {
				t.Fatalf("dumps(proto=%d) %s: %v", proto, Repr(v), err)
			}
			back, err := PickleLoads(data)
			if err != nil {
				t.Fatalf("loads(proto=%d) %s: %v", proto, Repr(v), err)
			}
			if Repr(back) != Repr(v) {
				t.Fatalf("roundtrip(proto=%d): %s -> %s", proto, Repr(v), Repr(back))
			}
		}
	}
}

// TestPickleCyclicList confirms a self-referential list round-trips: the loader
// must mutate the memoized list in place so the reconstructed cycle closes on
// the same object.
func TestPickleCyclicList(t *testing.T) {
	cyc := &listObject{}
	cyc.elts = []Object{cyc}
	data, err := PickleDumps(cyc, 5)
	if err != nil {
		t.Fatalf("dumps cyclic: %v", err)
	}
	// CPython: pickle.dumps(L, 5) for L=[]; L.append(L).
	if h := hex.EncodeToString(data); h != "80059506000000000000005d946800612e" {
		t.Fatalf("cyclic bytes\n got  %s\n want 80059506000000000000005d946800612e", h)
	}
	back, err := PickleLoads(data)
	if err != nil {
		t.Fatalf("loads cyclic: %v", err)
	}
	bl, ok := back.(*listObject)
	if !ok || len(bl.elts) != 1 || bl.elts[0] != Object(bl) {
		t.Fatalf("cyclic list did not close on itself: %#v", back)
	}
}

// mustSet builds a set from elts or fails the test.
func mustSet(t *testing.T, elts ...Object) Object {
	t.Helper()
	s, err := NewSet(elts)
	if err != nil {
		t.Fatalf("NewSet: %v", err)
	}
	return s
}

// mustFrozenset builds a frozenset from elts or fails the test.
func mustFrozenset(t *testing.T, elts ...Object) Object {
	t.Helper()
	f, err := NewFrozenset(elts)
	if err != nil {
		t.Fatalf("NewFrozenset: %v", err)
	}
	return f
}

// TestPickleDumpsSetsProtocol5 pins the exact protocol-5 bytes for sets and
// frozensets, whose element order is CPython's hash-table iteration order under
// the harness-pinned PYTHONHASHSEED=0. Each want is the hex of pickle.dumps from
// CPython 3.14.6 under that seed. Cases include a collision chain and a
// negative-int set (hash(-1) is -2, so -1 and -2 collide) to exercise the probe.
func TestPickleDumpsSetsProtocol5(t *testing.T) {
	i := func(n int64) Object { return NewInt(n) }
	shared := mustFrozenset(t, i(1), i(2))
	cases := []struct {
		name string
		obj  Object
		want string
	}{
		{"empty_set", mustSet(t), "80058f942e"},
		{"set123", mustSet(t, i(1), i(2), i(3)), "8005950b000000000000008f94284b014b024b03902e"},
		{"set_resize", mustSet(t, i(1), i(2), i(3), i(17), i(33)), "8005950f000000000000008f94284b014b024b034b214b11902e"},
		{"set_collide", mustSet(t, i(8), i(16), i(24), i(1)), "8005950d000000000000008f94284b084b104b184b01902e"},
		{"set_str", mustSet(t, NewStr("a"), NewStr("b"), NewStr("c")), "80059511000000000000008f94288c0163948c0161948c016294902e"},
		{"set_neg", mustSet(t, i(-1), i(-2), i(-3), i(-4), i(-5)), "8005951e000000000000008f94284afeffffff4afbffffff4afcffffff4afdffffff4affffffff902e"},
		{"empty_frozenset", mustFrozenset(t), "80059504000000000000002891942e"},
		{"frozenset123", mustFrozenset(t, i(1), i(2), i(3)), "8005950a00000000000000284b014b024b0391942e"},
		{"shared_frozenset", NewTuple([]Object{shared, shared}), "8005950c00000000000000284b014b029194680086942e"},
		{"list_of_sets", NewList([]Object{mustSet(t, i(1), i(2)), mustSet(t, i(3))}), "80059513000000000000005d94288f94284b014b02908f94284b0390652e"},
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

// TestPickleSetRoundTrip confirms the loader rebuilds sets and frozensets the
// pickler wrote, comparing by == (order-independent) across protocols 4 and 5.
func TestPickleSetRoundTrip(t *testing.T) {
	i := func(n int64) Object { return NewInt(n) }
	values := []Object{
		mustSet(t),
		mustSet(t, i(1), i(2), i(3), i(17), i(33)),
		mustSet(t, NewStr("x"), NewStr("y"), NewStr("z")),
		mustFrozenset(t),
		mustFrozenset(t, i(1), i(2), i(3)),
		NewList([]Object{mustSet(t, i(1), i(2)), mustSet(t, i(3), i(4))}),
		NewTuple([]Object{mustFrozenset(t, i(5)), mustFrozenset(t, i(5))}),
	}
	for _, proto := range []int{4, 5} {
		for _, v := range values {
			data, err := PickleDumps(v, proto)
			if err != nil {
				t.Fatalf("dumps(proto=%d) %s: %v", proto, Repr(v), err)
			}
			back, err := PickleLoads(data)
			if err != nil {
				t.Fatalf("loads(proto=%d) %s: %v", proto, Repr(v), err)
			}
			if !equals(v, back) {
				t.Fatalf("roundtrip(proto=%d): %s != %s", proto, Repr(v), Repr(back))
			}
		}
	}
}

// TestPickleSetProtocolRefusal confirms a set below protocol 4 is refused rather
// than emitting wrong bytes: CPython reaches the object-reduction protocol there,
// which this slice does not implement yet.
func TestPickleSetProtocolRefusal(t *testing.T) {
	for _, proto := range []int{2, 3} {
		if _, err := PickleDumps(mustSet(t, NewInt(1)), proto); err == nil {
			t.Fatalf("expected a set at protocol %d to be refused", proto)
		}
		if _, err := PickleDumps(mustFrozenset(t, NewInt(1)), proto); err == nil {
			t.Fatalf("expected a frozenset at protocol %d to be refused", proto)
		}
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
