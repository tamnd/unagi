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
