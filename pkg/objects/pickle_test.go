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

// TestPickleDumpsSetsReduction pins the exact protocol-2 and protocol-3 bytes for
// sets and frozensets, which have no native opcode there and pickle through the
// object-reduction protocol: builtins.set (or builtins.frozenset) applied to a
// list of the elements in set-iteration order. Protocol 2 remaps the module to
// the Python-2 name __builtin__ (fix_imports); protocol 3 keeps builtins. Each
// want is the hex of pickle.dumps from CPython 3.14.6 under PYTHONHASHSEED=0.
func TestPickleDumpsSetsReduction(t *testing.T) {
	i := func(n int64) Object { return NewInt(n) }
	cases := []struct {
		name   string
		obj    Object
		p2, p3 string
	}{
		{
			"empty_set", mustSet(t),
			"8002635f5f6275696c74696e5f5f0a7365740a71005d71018571025271032e",
			"8003636275696c74696e730a7365740a71005d71018571025271032e",
		},
		{
			"set123", mustSet(t, i(1), i(2), i(3)),
			"8002635f5f6275696c74696e5f5f0a7365740a71005d7101284b014b024b03658571025271032e",
			"8003636275696c74696e730a7365740a71005d7101284b014b024b03658571025271032e",
		},
		{
			"set_collide", mustSet(t, i(8), i(16), i(24), i(1)),
			"8002635f5f6275696c74696e5f5f0a7365740a71005d7101284b084b104b184b01658571025271032e",
			"8003636275696c74696e730a7365740a71005d7101284b084b104b184b01658571025271032e",
		},
		{
			"set_str", mustSet(t, NewStr("a"), NewStr("b"), NewStr("c")),
			"8002635f5f6275696c74696e5f5f0a7365740a71005d710128580100000063710258010000006171035801000000627104658571055271062e",
			"8003636275696c74696e730a7365740a71005d710128580100000063710258010000006171035801000000627104658571055271062e",
		},
		{
			"fset123", mustFrozenset(t, i(1), i(2), i(3)),
			"8002635f5f6275696c74696e5f5f0a66726f7a656e7365740a71005d7101284b014b024b03658571025271032e",
			"8003636275696c74696e730a66726f7a656e7365740a71005d7101284b014b024b03658571025271032e",
		},
		{
			"empty_fset", mustFrozenset(t),
			"8002635f5f6275696c74696e5f5f0a66726f7a656e7365740a71005d71018571025271032e",
			"8003636275696c74696e730a66726f7a656e7365740a71005d71018571025271032e",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for proto, want := range map[int]string{2: tc.p2, 3: tc.p3} {
				got, err := PickleDumps(tc.obj, proto)
				if err != nil {
					t.Fatalf("PickleDumps(%s, %d): %v", tc.name, proto, err)
				}
				if h := hex.EncodeToString(got); h != want {
					t.Fatalf("PickleDumps(%s, %d)\n got  %s\n want %s", tc.name, proto, h, want)
				}
			}
		})
	}
}

// TestPickleDumpsGlobalStackGlobal pins the STACK_GLOBAL form saveGlobal writes
// under protocol 4+, the branch the native set path never reaches. The bytes are
// pickle.dumps(set, protocol=4/5) from CPython: the module and qualname go out as
// memoized SHORT_BINUNICODE strings, then STACK_GLOBAL, then a memoize.
func TestPickleDumpsGlobalStackGlobal(t *testing.T) {
	for _, proto := range []int{4, 5} {
		p := &pickler{memo: map[Object]int{}, proto: proto, bin: true}
		p.framer.out = append(p.framer.out, opProto, byte(proto))
		p.framer.startFraming()
		if err := p.saveGlobal("builtins", "set"); err != nil {
			t.Fatalf("saveGlobal(proto=%d): %v", proto, err)
		}
		p.framer.write(opStop)
		p.framer.endFraming()
		want := "80" + hexByte(proto) + "9514000000000000008c086275696c74696e73948c037365749493942e"
		if h := hex.EncodeToString(p.framer.out); h != want {
			t.Fatalf("saveGlobal STACK_GLOBAL(proto=%d)\n got  %s\n want %s", proto, h, want)
		}
	}
}

func hexByte(n int) string { return hex.EncodeToString([]byte{byte(n)}) }

// TestPickleSetReductionRoundTrip confirms the loader rebuilds sets and
// frozensets pickled through the reduction protocol at protocols 2 and 3.
func TestPickleSetReductionRoundTrip(t *testing.T) {
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
	for _, proto := range []int{2, 3} {
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

// mustPickleClass builds a plain object-rooted class in the __main__ module and
// registers it, so the pickle names it the way a class defined in the running
// script is named and the loader's find_class can resolve it back.
func mustPickleClass(t *testing.T, name string) *classObject {
	t.Helper()
	c, err := NewClass(name, name, []Object{nil}, []string{"__module__"}, []Object{NewStr("__main__")}, nil, nil)
	if err != nil {
		t.Fatalf("NewClass(%s): %v", name, err)
	}
	return c.(*classObject)
}

// newPickleInstance builds an instance of c with the given name/value attribute
// pairs in order, the __dict__ the default reduction pickles.
func newPickleInstance(c *classObject, pairs ...Object) *instanceObject {
	o := &instanceObject{cls: c, attrs: newAttrs()}
	for i := 0; i < len(pairs); i += 2 {
		_ = o.attrs.set(pairs[i], pairs[i+1])
	}
	return o
}

// TestPickleDumpsInstance pins the default instance reduction byte for byte
// against CPython 3.14: NEWOBJ over the class global with the __dict__ restored
// by BUILD, and an attribute-free instance that stops right after NEWOBJ with no
// BUILD. The vectors are pickle.dumps of the equivalent classes under PYTHONHASHSEED=0.
func TestPickleDumpsInstance(t *testing.T) {
	P := mustPickleClass(t, "P")
	E := mustPickleClass(t, "E")
	point := newPickleInstance(P, NewStr("x"), NewInt(1), NewStr("y"), NewStr("hi"))
	cases := []struct {
		name  string
		obj   Object
		proto int
		want  string
	}{
		{"point_p2", point, 2, "8002635f5f6d61696e5f5f0a500a7100298171017d71022858010000007871034b01580100000079710458020000006869710575622e"},
		{"point_p4", point, 4, "80049529000000000000008c085f5f6d61696e5f5f948c01509493942981947d94288c0178944b018c0179948c0268699475622e"},
		{"empty_p2", newPickleInstance(E), 2, "8002635f5f6d61696e5f5f0a450a7100298171012e"},
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

// TestPickleInstanceRoundTrip confirms the loader rebuilds a user instance at
// every binary protocol: NEWOBJ resolves the class through the registry, and
// BUILD restores its __dict__ in order, so the reconstructed attributes match.
func TestPickleInstanceRoundTrip(t *testing.T) {
	C := mustPickleClass(t, "RT")
	for _, proto := range []int{2, 3, 4, 5} {
		inst := newPickleInstance(C, NewStr("a"), NewInt(7), NewStr("b"), NewStr("hi"))
		data, err := PickleDumps(inst, proto)
		if err != nil {
			t.Fatalf("dumps(proto=%d): %v", proto, err)
		}
		back, err := PickleLoads(data)
		if err != nil {
			t.Fatalf("loads(proto=%d): %v", proto, err)
		}
		bi, ok := back.(*instanceObject)
		if !ok {
			t.Fatalf("loads(proto=%d) returned %s, want instance", proto, back.TypeName())
		}
		if bi.cls != C {
			t.Fatalf("loads(proto=%d) rebuilt class %p, want %p", proto, bi.cls, C)
		}
		for _, kv := range []struct {
			k string
			v Object
		}{{"a", NewInt(7)}, {"b", NewStr("hi")}} {
			got, ok := bi.attrGet(kv.k)
			if !ok || !equals(got, kv.v) {
				t.Fatalf("loads(proto=%d) attr %s = %v (ok=%v), want %s", proto, kv.k, got, ok, Repr(kv.v))
			}
		}
	}
}

// TestPickleEmptyInstanceRoundTrip confirms an attribute-free instance, pickled
// with no BUILD, comes back with an empty __dict__.
func TestPickleEmptyInstanceRoundTrip(t *testing.T) {
	C := mustPickleClass(t, "RTEmpty")
	for _, proto := range []int{2, 3, 4, 5} {
		data, err := PickleDumps(newPickleInstance(C), proto)
		if err != nil {
			t.Fatalf("dumps(proto=%d): %v", proto, err)
		}
		back, err := PickleLoads(data)
		if err != nil {
			t.Fatalf("loads(proto=%d): %v", proto, err)
		}
		bi, ok := back.(*instanceObject)
		if !ok || bi.cls != C {
			t.Fatalf("loads(proto=%d) returned %s, want RTEmpty instance", proto, back.TypeName())
		}
		if len(bi.attrs.entries) != 0 {
			t.Fatalf("loads(proto=%d) __dict__ = %d entries, want 0", proto, len(bi.attrs.entries))
		}
	}
}

// mustPickleFunction builds a module-level function under the given qualname and
// registers it, the way a compiled module-level def does, so it pickles by
// qualified name and the loader resolves it back.
func mustPickleFunction(qual string) *functionObject {
	fn := NewFunctionT(qual, nil, nil, func(_ *Thread, args []Object) (Object, error) {
		return NewStr("ok"), nil
	}).(*functionObject)
	RegisterPickleFunction(fn)
	return fn
}

// TestPickleDumpsGlobal pins the bare global reference a module-level function
// and a class object pickle as, byte for byte against CPython 3.14: GLOBAL plus a
// BINPUT memo at protocol 2, SHORT_BINUNICODE names plus STACK_GLOBAL inside a
// frame at protocol 4+.
func TestPickleDumpsGlobal(t *testing.T) {
	greet := mustPickleFunction("greetfn")
	Pt := mustPickleClass(t, "PtGlobal")
	cases := []struct {
		name  string
		obj   Object
		proto int
		want  string
	}{
		{"fn_p2", greet, 2, "8002635f5f6d61696e5f5f0a6772656574666e0a71002e"},
		{"fn_p4", greet, 4, "80049518000000000000008c085f5f6d61696e5f5f948c076772656574666e9493942e"},
		{"cls_p2", Pt, 2, "8002635f5f6d61696e5f5f0a5074476c6f62616c0a71002e"},
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

// TestPickleGlobalRoundTrip confirms a module-level function and a class object
// come back as the very same registered object at every binary protocol.
func TestPickleGlobalRoundTrip(t *testing.T) {
	fn := mustPickleFunction("rtGlobalFn")
	cls := mustPickleClass(t, "RTGlobalCls")
	for _, proto := range []int{2, 3, 4, 5} {
		for _, want := range []Object{fn, cls} {
			data, err := PickleDumps(want, proto)
			if err != nil {
				t.Fatalf("dumps(proto=%d): %v", proto, err)
			}
			back, err := PickleLoads(data)
			if err != nil {
				t.Fatalf("loads(proto=%d): %v", proto, err)
			}
			if back != want {
				t.Fatalf("loads(proto=%d) = %p, want %p", proto, back, want)
			}
		}
	}
}

// TestPickleGlobalMemo confirms a global referenced twice is written once and
// fetched back from the memo, so both slots recover the identical object.
func TestPickleGlobalMemo(t *testing.T) {
	fn := mustPickleFunction("memoFn")
	// CPython: pickle.dumps([memoFn, memoFn], 4) — the second element is a BINGET.
	data, err := PickleDumps(NewList([]Object{fn, fn}), 4)
	if err != nil {
		t.Fatalf("dumps: %v", err)
	}
	want := "8004951d000000000000005d94288c085f5f6d61696e5f5f948c066d656d6f466e9493946803652e"
	if h := hex.EncodeToString(data); h != want {
		t.Fatalf("dumps([fn, fn])\n got  %s\n want %s", h, want)
	}
}

// TestPickleLocalGlobalRefused confirms a function or class not reachable by
// qualified name — a lambda, a nested def, a class defined inside a function —
// is refused with a PicklingError rather than pickled as a reference that would
// not resolve the same way CPython's would.
func TestPickleLocalGlobalRefused(t *testing.T) {
	local := NewFunctionT("outer.<locals>.inner", nil, nil, func(_ *Thread, _ []Object) (Object, error) {
		return None, nil
	})
	if _, err := PickleDumps(local, 5); err == nil {
		t.Fatalf("PickleDumps(local function) = nil error, want PicklingError")
	}
	// An unregistered plain function (never bound as a module attribute) is also
	// unreachable by name and is refused.
	orphan := NewFunctionT("orphanFn", nil, nil, func(_ *Thread, _ []Object) (Object, error) {
		return None, nil
	})
	if _, err := PickleDumps(orphan, 5); err == nil {
		t.Fatalf("PickleDumps(unregistered function) = nil error, want PicklingError")
	}
}

// mustReduceClass builds a class whose instances pickle through a custom
// __reduce__: the supplied closure produces the reduction tuple from self,
// exactly the value a Python __reduce__ method returns.
func mustReduceClass(t *testing.T, name string, reduce func(self *instanceObject) (Object, error)) *classObject {
	t.Helper()
	red := NewFunctionT("__reduce__", []Param{{Name: "self", Kind: ParamPlain}}, nil, func(_ *Thread, args []Object) (Object, error) {
		self, ok := args[0].(*instanceObject)
		if !ok {
			return nil, Raise(TypeError, "__reduce__ self is not an instance")
		}
		return reduce(self)
	})
	c, err := NewClass(name, name, []Object{nil},
		[]string{"__module__", "__reduce__"},
		[]Object{NewStr("__main__"), red}, nil, nil)
	if err != nil {
		t.Fatalf("NewClass(%s): %v", name, err)
	}
	return c.(*classObject)
}

// TestPickleDumpsReduce pins a plain two-element reduction byte for byte against
// CPython 3.14: a class whose __reduce__ returns (rebuild, (self.x, self.y))
// pickles as the rebuild global, the argument tuple, and REDUCE, and the loader
// applies the registered rebuild function to reconstruct the instance.
func TestPickleDumpsReduce(t *testing.T) {
	var coord *classObject
	rebuild := NewFunctionT("rebuild", []Param{{Name: "x", Kind: ParamPlain}, {Name: "y", Kind: ParamPlain}}, nil, func(_ *Thread, args []Object) (Object, error) {
		o := &instanceObject{cls: coord, attrs: newAttrs()}
		_ = o.attrs.set(NewStr("x"), args[0])
		_ = o.attrs.set(NewStr("y"), args[1])
		return o, nil
	}).(*functionObject)
	RegisterPickleFunction(rebuild)

	coord = mustReduceClass(t, "Coord", func(self *instanceObject) (Object, error) {
		x, _ := self.attrGet("x")
		y, _ := self.attrGet("y")
		return NewTuple([]Object{rebuild, NewTuple([]Object{x, y})}), nil
	})

	inst := &instanceObject{cls: coord, attrs: newAttrs()}
	_ = inst.attrs.set(NewStr("x"), NewInt(3))
	_ = inst.attrs.set(NewStr("y"), NewInt(4))

	// CPython: pickle.dumps(Coord(3, 4), 2) with Coord.__reduce__ = (rebuild, (x, y)).
	want := "8002635f5f6d61696e5f5f0a72656275696c640a71004b034b048671015271022e"
	got, err := PickleDumps(inst, 2)
	if err != nil {
		t.Fatalf("PickleDumps: %v", err)
	}
	if h := hex.EncodeToString(got); h != want {
		t.Fatalf("PickleDumps(reduce)\n got  %s\n want %s", h, want)
	}

	back, err := PickleLoads(got)
	if err != nil {
		t.Fatalf("PickleLoads: %v", err)
	}
	bi, ok := back.(*instanceObject)
	if !ok || bi.cls != coord {
		t.Fatalf("PickleLoads returned %s, want Coord instance", back.TypeName())
	}
	if x, _ := bi.attrGet("x"); !equals(x, NewInt(3)) {
		t.Fatalf("rebuilt x = %v, want 3", x)
	}
	if y, _ := bi.attrGet("y"); !equals(y, NewInt(4)) {
		t.Fatalf("rebuilt y = %v, want 4", y)
	}
}

// mustNewargsClass builds a class with a custom __new__ and a __getnewargs__ that
// round-trips the arguments __new__ needs, optionally with a __getstate__. The
// __new__ allocates a bare instance and applies each (name, arg) pair as an
// attribute so the reconstructed object carries the constructor's values.
func mustNewargsClass(t *testing.T, name string, argNames []string, getstate Object) *classObject {
	t.Helper()
	params := append([]Param{{Name: "cls", Kind: ParamPlain}}, func() []Param {
		ps := make([]Param, len(argNames))
		for i, n := range argNames {
			ps[i] = Param{Name: n, Kind: ParamPlain}
		}
		return ps
	}()...)
	var cls *classObject
	newFn := NewFunctionT(name+".__new__", params, nil, func(_ *Thread, args []Object) (Object, error) {
		o := &instanceObject{cls: cls, attrs: newAttrs()}
		for i, n := range argNames {
			_ = o.attrs.set(NewStr(n), args[i+1])
		}
		return o, nil
	})
	getnewargs := NewFunctionT(name+".__getnewargs__", []Param{{Name: "self", Kind: ParamPlain}}, nil, func(_ *Thread, args []Object) (Object, error) {
		self := args[0].(*instanceObject)
		vals := make([]Object, len(argNames))
		for i, n := range argNames {
			vals[i], _ = self.attrGet(n)
		}
		return NewTuple(vals), nil
	})
	names := []string{"__module__", "__new__", "__getnewargs__"}
	vals := []Object{NewStr("__main__"), newFn, getnewargs}
	if getstate != nil {
		names = append(names, "__getstate__")
		vals = append(vals, getstate)
	}
	c, err := NewClass(name, name, []Object{nil}, names, vals, nil, nil)
	if err != nil {
		t.Fatalf("NewClass(%s): %v", name, err)
	}
	cls = c.(*classObject)
	registerPickleClass(cls)
	return cls
}

// TestPickleDumpsNewargs pins the __getnewargs__ NEWOBJ path byte for byte against
// CPython 3.14: a class defining __new__ and __getnewargs__ pickles the class
// global, the new-arguments tuple, NEWOBJ, then the __dict__ state through BUILD,
// and the loader rebuilds through cls.__new__(cls, *args).
func TestPickleDumpsNewargs(t *testing.T) {
	vec := mustNewargsClass(t, "Vec", []string{"x", "y"}, nil)

	inst := &instanceObject{cls: vec, attrs: newAttrs()}
	_ = inst.attrs.set(NewStr("x"), NewInt(3))
	_ = inst.attrs.set(NewStr("y"), NewInt(4))

	// CPython: pickle.dumps(Vec(3, 4), 2) with __new__ setting x, y and
	// __getnewargs__ returning (x, y); NEWOBJ carries (3, 4), BUILD restores __dict__.
	want := "8002635f5f6d61696e5f5f0a5665630a71004b034b048671018171027d71032858010000007871044b0358010000007971054b0475622e"
	got, err := PickleDumps(inst, 2)
	if err != nil {
		t.Fatalf("PickleDumps: %v", err)
	}
	if h := hex.EncodeToString(got); h != want {
		t.Fatalf("PickleDumps(newargs)\n got  %s\n want %s", h, want)
	}

	back, err := PickleLoads(got)
	if err != nil {
		t.Fatalf("PickleLoads: %v", err)
	}
	bi, ok := back.(*instanceObject)
	if !ok || bi.cls != vec {
		t.Fatalf("PickleLoads returned %s, want Vec instance", back.TypeName())
	}
	if x, _ := bi.attrGet("x"); !equals(x, NewInt(3)) {
		t.Fatalf("rebuilt x = %v, want 3", x)
	}
	if y, _ := bi.attrGet("y"); !equals(y, NewInt(4)) {
		t.Fatalf("rebuilt y = %v, want 4", y)
	}
}

// TestPickleNewargsGetstateNone confirms a __getstate__ returning None suppresses
// BUILD: the whole value rides in the __new__ arguments, so the pickle stops right
// after NEWOBJ and no state is written even though the instance holds a __dict__.
func TestPickleNewargsGetstateNone(t *testing.T) {
	getstate := NewFunctionT("Frozen.__getstate__", []Param{{Name: "self", Kind: ParamPlain}}, nil, func(_ *Thread, _ []Object) (Object, error) {
		return None, nil
	})
	frozen := mustNewargsClass(t, "Frozen", []string{"value"}, getstate)

	inst := &instanceObject{cls: frozen, attrs: newAttrs()}
	_ = inst.attrs.set(NewStr("value"), NewStr("hi"))

	// CPython: pickle.dumps(Frozen("hi"), 2); __getstate__ returns None so there is
	// no trailing BUILD, only the class global, the ("hi",) tuple, NEWOBJ, STOP.
	want := "8002635f5f6d61696e5f5f0a46726f7a656e0a71005802000000686971018571028171032e"
	got, err := PickleDumps(inst, 2)
	if err != nil {
		t.Fatalf("PickleDumps: %v", err)
	}
	if h := hex.EncodeToString(got); h != want {
		t.Fatalf("PickleDumps(getstate=None)\n got  %s\n want %s", h, want)
	}

	back, err := PickleLoads(got)
	if err != nil {
		t.Fatalf("PickleLoads: %v", err)
	}
	bi, ok := back.(*instanceObject)
	if !ok || bi.cls != frozen {
		t.Fatalf("PickleLoads returned %s, want Frozen instance", back.TypeName())
	}
	if v, _ := bi.attrGet("value"); !equals(v, NewStr("hi")) {
		t.Fatalf("rebuilt value = %v, want 'hi'", v)
	}
}

// mustNewargsExClass builds a class with a custom __new__ taking one positional
// and one keyword-only argument, a __getnewargs_ex__ that round-trips them, and a
// __getstate__ returning None so the whole value rides in NEWOBJ_EX.
func mustNewargsExClass(t *testing.T, name string) *classObject {
	t.Helper()
	var cls *classObject
	newFn := NewFunctionT(name+".__new__",
		[]Param{{Name: "cls", Kind: ParamPlain}, {Name: "x", Kind: ParamPlain}, {Name: "y", Kind: ParamKwOnly}},
		nil, func(_ *Thread, args []Object) (Object, error) {
			o := &instanceObject{cls: cls, attrs: newAttrs()}
			_ = o.attrs.set(NewStr("x"), args[1])
			_ = o.attrs.set(NewStr("y"), args[2])
			return o, nil
		})
	getnewargsEx := NewFunctionT(name+".__getnewargs_ex__", []Param{{Name: "self", Kind: ParamPlain}}, nil, func(_ *Thread, args []Object) (Object, error) {
		self := args[0].(*instanceObject)
		x, _ := self.attrGet("x")
		y, _ := self.attrGet("y")
		kw, err := NewDict([]Object{NewStr("y")}, []Object{y})
		if err != nil {
			return nil, err
		}
		return NewTuple([]Object{NewTuple([]Object{x}), kw}), nil
	})
	getstate := NewFunctionT(name+".__getstate__", []Param{{Name: "self", Kind: ParamPlain}}, nil, func(_ *Thread, _ []Object) (Object, error) {
		return None, nil
	})
	c, err := NewClass(name, name, []Object{nil},
		[]string{"__module__", "__new__", "__getnewargs_ex__", "__getstate__"},
		[]Object{NewStr("__main__"), newFn, getnewargsEx, getstate}, nil, nil)
	if err != nil {
		t.Fatalf("NewClass(%s): %v", name, err)
	}
	cls = c.(*classObject)
	registerPickleClass(cls)
	return cls
}

// TestPickleDumpsNewargsEx pins the __getnewargs_ex__ NEWOBJ_EX path byte for byte
// against CPython 3.14: a class defining __getnewargs_ex__ pickles the class
// global, the positional argument tuple, the keyword dict, and NEWOBJ_EX, and the
// loader rebuilds through cls.__new__(cls, *args, **kwargs).
func TestPickleDumpsNewargsEx(t *testing.T) {
	point := mustNewargsExClass(t, "Point")

	inst := &instanceObject{cls: point, attrs: newAttrs()}
	_ = inst.attrs.set(NewStr("x"), NewInt(3))
	_ = inst.attrs.set(NewStr("y"), NewInt(4))

	// CPython: pickle.dumps(Point(3, y=4), 4); NEWOBJ_EX carries ((3,), {"y": 4})
	// and __getstate__ returning None stops the pickle right after it.
	want := "80049525000000000000008c085f5f6d61696e5f5f948c05506f696e749493944b0385947d948c0179944b047392942e"
	got, err := PickleDumps(inst, 4)
	if err != nil {
		t.Fatalf("PickleDumps: %v", err)
	}
	if h := hex.EncodeToString(got); h != want {
		t.Fatalf("PickleDumps(newargs_ex)\n got  %s\n want %s", h, want)
	}

	back, err := PickleLoads(got)
	if err != nil {
		t.Fatalf("PickleLoads: %v", err)
	}
	bi, ok := back.(*instanceObject)
	if !ok || bi.cls != point {
		t.Fatalf("PickleLoads returned %s, want Point instance", back.TypeName())
	}
	if x, _ := bi.attrGet("x"); !equals(x, NewInt(3)) {
		t.Fatalf("rebuilt x = %v, want 3", x)
	}
	if y, _ := bi.attrGet("y"); !equals(y, NewInt(4)) {
		t.Fatalf("rebuilt y = %v, want 4 (keyword argument)", y)
	}
}

// TestPickleNewargsExRefusedBelowProto4 confirms a class with __getnewargs_ex__ is
// refused under protocols 2 and 3, which have no NEWOBJ_EX opcode; the
// functools.partial fallback CPython uses there is a later slice.
func TestPickleNewargsExRefusedBelowProto4(t *testing.T) {
	point := mustNewargsExClass(t, "PointLow")
	inst := &instanceObject{cls: point, attrs: newAttrs()}
	_ = inst.attrs.set(NewStr("x"), NewInt(3))
	_ = inst.attrs.set(NewStr("y"), NewInt(4))

	for _, proto := range []int{2, 3} {
		if _, err := PickleDumps(inst, proto); err == nil {
			t.Fatalf("PickleDumps(proto=%d) succeeded, want refusal", proto)
		}
	}
}

// TestPickleReduceStateRoundTrip covers the three-element reduction: the state
// dict is saved after REDUCE and applied by BUILD, so the reconstructed instance
// carries the attributes the partial constructor left out.
func TestPickleReduceStateRoundTrip(t *testing.T) {
	var box *classObject
	makeBox := NewFunctionT("makeBox", []Param{{Name: "a", Kind: ParamPlain}}, nil, func(_ *Thread, args []Object) (Object, error) {
		o := &instanceObject{cls: box, attrs: newAttrs()}
		_ = o.attrs.set(NewStr("a"), args[0])
		_ = o.attrs.set(NewStr("b"), None)
		return o, nil
	}).(*functionObject)
	RegisterPickleFunction(makeBox)

	box = mustReduceClass(t, "BoxRT", func(self *instanceObject) (Object, error) {
		a, _ := self.attrGet("a")
		b, _ := self.attrGet("b")
		state, err := NewDict([]Object{NewStr("b")}, []Object{b})
		if err != nil {
			return nil, err
		}
		return NewTuple([]Object{makeBox, NewTuple([]Object{a}), state}), nil
	})

	for _, proto := range []int{2, 3, 4, 5} {
		inst := &instanceObject{cls: box, attrs: newAttrs()}
		_ = inst.attrs.set(NewStr("a"), NewStr("hi"))
		_ = inst.attrs.set(NewStr("b"), NewInt(9))
		data, err := PickleDumps(inst, proto)
		if err != nil {
			t.Fatalf("dumps(proto=%d): %v", proto, err)
		}
		back, err := PickleLoads(data)
		if err != nil {
			t.Fatalf("loads(proto=%d): %v", proto, err)
		}
		bi, ok := back.(*instanceObject)
		if !ok || bi.cls != box {
			t.Fatalf("loads(proto=%d) returned %s, want BoxRT instance", proto, back.TypeName())
		}
		if a, _ := bi.attrGet("a"); !equals(a, NewStr("hi")) {
			t.Fatalf("loads(proto=%d) a = %v, want 'hi'", proto, a)
		}
		if b, _ := bi.attrGet("b"); !equals(b, NewInt(9)) {
			t.Fatalf("loads(proto=%d) b = %v, want 9 (BUILD state)", proto, b)
		}
	}
}

// TestPickleReduceExProtocol confirms a class defining __reduce_ex__ is dispatched
// with the protocol, and that a reduction naming the class itself as the callable
// pickles the class global and rebuilds by calling it. The vector is CPython's
// pickle.dumps(Temperature(21), 2).
func TestPickleReduceExProtocol(t *testing.T) {
	var seen int
	redEx := NewFunctionT("__reduce_ex__", []Param{{Name: "self", Kind: ParamPlain}, {Name: "protocol", Kind: ParamPlain}}, nil, func(_ *Thread, args []Object) (Object, error) {
		self := args[0].(*instanceObject)
		if p, ok := args[1].(*intObject); ok {
			seen = int(p.v)
		}
		c, _ := self.attrGet("celsius")
		return NewTuple([]Object{self.cls, NewTuple([]Object{c})}), nil
	})
	initFn := NewFunctionT("__init__", []Param{{Name: "self", Kind: ParamPlain}, {Name: "celsius", Kind: ParamPlain}}, nil, func(_ *Thread, args []Object) (Object, error) {
		self := args[0].(*instanceObject)
		_ = self.attrs.set(NewStr("celsius"), args[1])
		return None, nil
	})
	cobj, err := NewClass("Temperature", "Temperature", []Object{nil},
		[]string{"__module__", "__reduce_ex__", "__init__"},
		[]Object{NewStr("__main__"), redEx, initFn}, nil, nil)
	if err != nil {
		t.Fatalf("NewClass: %v", err)
	}
	temp := cobj.(*classObject)

	inst := &instanceObject{cls: temp, attrs: newAttrs()}
	_ = inst.attrs.set(NewStr("celsius"), NewInt(21))

	want := "8002635f5f6d61696e5f5f0a54656d70657261747572650a71004b158571015271022e"
	got, err := PickleDumps(inst, 2)
	if err != nil {
		t.Fatalf("PickleDumps: %v", err)
	}
	if h := hex.EncodeToString(got); h != want {
		t.Fatalf("PickleDumps(reduce_ex)\n got  %s\n want %s", h, want)
	}
	if seen != 2 {
		t.Fatalf("__reduce_ex__ received protocol %d, want 2", seen)
	}

	back, err := PickleLoads(got)
	if err != nil {
		t.Fatalf("PickleLoads: %v", err)
	}
	bi, ok := back.(*instanceObject)
	if !ok || bi.cls != temp {
		t.Fatalf("PickleLoads returned %s, want Temperature instance", back.TypeName())
	}
	if c, _ := bi.attrGet("celsius"); !equals(c, NewInt(21)) {
		t.Fatalf("rebuilt celsius = %v, want 21", c)
	}
}
