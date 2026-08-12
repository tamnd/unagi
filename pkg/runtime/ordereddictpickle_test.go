package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// mustOrderedDict builds an OrderedDict from parallel key/value slices. The
// collections init registers collections.OrderedDict for pickling at load, so no
// import is needed to reach the registered type through a reduction.
func mustOrderedDict(t *testing.T, keys, vals []objects.Object) objects.Object {
	t.Helper()
	od, err := objects.NewOrderedDict(keys, vals)
	if err != nil {
		t.Fatalf("NewOrderedDict: %v", err)
	}
	return od
}

// TestOrderedDictPickleRoundTrip confirms an OrderedDict survives dumps/loads at
// every binary protocol, coming back an OrderedDict with the same pairs in the
// same order. It reduces through the five-tuple OrderedDict.__reduce_ex__ emits,
// whose item iterator the pickler replays as setitems onto the reconstructed
// mapping.
func TestOrderedDictPickleRoundTrip(t *testing.T) {
	keys := []objects.Object{objects.NewStr("z"), objects.NewStr("a"), objects.NewStr("m")}
	vals := []objects.Object{objects.NewInt(1), objects.NewInt(2), objects.NewInt(3)}
	od := mustOrderedDict(t, keys, vals)
	for _, proto := range []int{2, 3, 4, 5} {
		data, err := objects.PickleDumps(od, proto)
		if err != nil {
			t.Fatalf("dumps(proto=%d): %v", proto, err)
		}
		back, err := objects.PickleLoads(data)
		if err != nil {
			t.Fatalf("loads(proto=%d): %v", proto, err)
		}
		if back.TypeName() != "OrderedDict" {
			t.Fatalf("loads(proto=%d) = %s, want an OrderedDict", proto, back.TypeName())
		}
		eqObj, err := objects.Compare(objects.OpEq, back, od)
		if err != nil {
			t.Fatalf("compare(proto=%d): %v", proto, err)
		}
		if eq, _ := objects.TruthOf(eqObj); !eq {
			t.Fatalf("loads(proto=%d) = %s, want %s", proto, objects.Repr(back), objects.Repr(od))
		}
		// The order rides through: the reconstructed dict reprs its pairs in the
		// original z, a, m order (OrderedDict equality is order-sensitive, so the
		// compare above already leans on this).
		if got, want := objects.Repr(back), "OrderedDict({'z': 1, 'a': 2, 'm': 3})"; got != want {
			t.Fatalf("loads(proto=%d) = %s, want %s", proto, got, want)
		}
	}
}
