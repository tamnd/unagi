package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// mustDeque builds a deque with the given elements and maxlen. The collections
// package init registers collections.deque for pickling at load, so no import
// is needed to reach the registered type through a reduction.
func mustDeque(t *testing.T, elts []objects.Object, maxlen int) objects.Object {
	t.Helper()
	return objects.NewDeque(elts, maxlen)
}

// TestDequePickleRoundTrip confirms a deque survives dumps/loads at every binary
// protocol, coming back a deque with the same elements and maxlen. It reduces
// through the four-tuple deque.__reduce_ex__ emits, whose element iterator the
// pickler replays as appends onto the reconstructed deque.
func TestDequePickleRoundTrip(t *testing.T) {
	cases := []struct {
		name   string
		elts   []objects.Object
		maxlen int
	}{
		{"bounded", []objects.Object{objects.NewInt(1), objects.NewInt(2), objects.NewInt(3)}, 5},
		{"unbounded", []objects.Object{objects.NewInt(7), objects.NewInt(8)}, -1},
		{"empty", nil, 2},
	}
	for _, tc := range cases {
		d := mustDeque(t, tc.elts, tc.maxlen)
		for _, proto := range []int{2, 3, 4, 5} {
			data, err := objects.PickleDumps(d, proto)
			if err != nil {
				t.Fatalf("%s dumps(proto=%d): %v", tc.name, proto, err)
			}
			back, err := objects.PickleLoads(data)
			if err != nil {
				t.Fatalf("%s loads(proto=%d): %v", tc.name, proto, err)
			}
			if back.TypeName() != "collections.deque" {
				t.Fatalf("%s loads(proto=%d) = %s, want a deque", tc.name, proto, back.TypeName())
			}
			eqObj, err := objects.Compare(objects.OpEq, back, d)
			if err != nil {
				t.Fatalf("%s compare(proto=%d): %v", tc.name, proto, err)
			}
			if eq, _ := objects.TruthOf(eqObj); !eq {
				t.Fatalf("%s loads(proto=%d) = %s, want %s", tc.name, proto, objects.Repr(back), objects.Repr(d))
			}
			ml, err := objects.LoadAttr(back, "maxlen")
			if err != nil {
				t.Fatalf("%s maxlen(proto=%d): %v", tc.name, proto, err)
			}
			if tc.maxlen < 0 {
				if ml != objects.None {
					t.Fatalf("%s loads(proto=%d) maxlen = %s, want None", tc.name, proto, objects.Repr(ml))
				}
			} else if v, _ := objects.AsIntValue(ml); int(v) != tc.maxlen {
				t.Fatalf("%s loads(proto=%d) maxlen = %s, want %d", tc.name, proto, objects.Repr(ml), tc.maxlen)
			}
		}
	}
}
