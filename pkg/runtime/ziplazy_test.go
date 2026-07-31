package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// countingIter yields 0, 1, 2, ... forever and records how many times it was
// pulled, so a test can prove zip stops advancing it the instant a shorter
// input runs out.
type countingIter struct {
	pulls int
	next  int64
}

func (c *countingIter) TypeName() string                   { return "counting" }
func (c *countingIter) Iterate() (objects.Iterator, error) { return c, nil }
func (c *countingIter) Next() (objects.Object, bool, error) {
	c.pulls++
	v := objects.NewInt(c.next)
	c.next++
	return v, true, nil
}

// TestZipStopsWithoutOverconsuming pins the guarantee heapq.nsmallest relies on:
// once the shorter input (here the two-element list, in position 0) is spent,
// zip must not pull from the inputs after it. Draining zip([a, b], counter) pulls
// counter exactly twice -- never a third time for the round the list ends.
func TestZipStopsWithoutOverconsuming(t *testing.T) {
	counter := &countingIter{}
	z, err := Zip(objs(newList(i(10), i(20)), counter))
	if err != nil {
		t.Fatalf("zip: %v", err)
	}
	lst, err := ListOf(objs(z))
	if err != nil {
		t.Fatalf("list(zip): %v", err)
	}
	if got := objects.Repr(lst); got != "[(10, 0), (20, 1)]" {
		t.Errorf("zip rows = %s, want [(10, 0), (20, 1)]", got)
	}
	if counter.pulls != 2 {
		t.Errorf("counter pulled %d times, want 2 (no over-consumption)", counter.pulls)
	}
}

// cursorIter walks a fixed slice once and, being its own Iterate target, hands
// zip a stateful iterator whose leftover elements a test can inspect afterward.
type cursorIter struct {
	elts []objects.Object
	pos  int
}

func (c *cursorIter) TypeName() string                   { return "cursor" }
func (c *cursorIter) Iterate() (objects.Iterator, error) { return c, nil }
func (c *cursorIter) Next() (objects.Object, bool, error) {
	if c.pos >= len(c.elts) {
		return nil, false, nil
	}
	v := c.elts[c.pos]
	c.pos++
	return v, true, nil
}

// TestZipLeavesTailUnconsumed checks the same guarantee from the input's side: a
// shared iterator zipped against a shorter one keeps the elements zip never
// reached, so a following loop over it sees them.
func TestZipLeavesTailUnconsumed(t *testing.T) {
	it := &cursorIter{elts: []objects.Object{i(1), i(2), i(3), i(4), i(5)}}
	z, err := Zip(objs(newList(i(0), i(0)), it))
	if err != nil {
		t.Fatalf("zip: %v", err)
	}
	if _, err := ListOf(objs(z)); err != nil {
		t.Fatalf("list(zip): %v", err)
	}
	// zip pulled the list (arg 0) to exhaustion after two rows, so it pulled the
	// shared iterator exactly twice; 3, 4, 5 remain.
	var rest []int64
	for {
		v, ok, err := it.Next()
		if err != nil {
			t.Fatalf("drain: %v", err)
		}
		if !ok {
			break
		}
		n, _ := objects.AsInt(v)
		rest = append(rest, n)
	}
	if len(rest) != 3 || rest[0] != 3 || rest[2] != 5 {
		t.Errorf("remaining after zip = %v, want [3 4 5]", rest)
	}
}

// TestZipStrictErrors pins the four length-mismatch reports: a later input
// running short blames the earlier ones, the first input running short only errs
// when a later one is longer, and the wording tracks how many inputs precede it.
func TestZipStrictErrors(t *testing.T) {
	yes := objects.True
	cases := []struct {
		name string
		args []objects.Object
		want string
	}{
		{"2nd shorter", objs(newList(i(1), i(2), i(3)), newList(i(1), i(2))),
			"ValueError: zip() argument 2 is shorter than argument 1"},
		{"2nd longer", objs(newList(i(1), i(2)), newList(i(1), i(2), i(3))),
			"ValueError: zip() argument 2 is longer than argument 1"},
		{"3rd shorter", objs(newList(i(1), i(2), i(3)), newList(i(1), i(2), i(3)), newList(i(1), i(2))),
			"ValueError: zip() argument 3 is shorter than arguments 1-2"},
		{"2nd longer of three", objs(newList(i(1), i(2)), newList(i(1), i(2), i(3)), newList(i(1), i(2), i(3))),
			"ValueError: zip() argument 2 is longer than argument 1"},
	}
	for _, tc := range cases {
		z, err := ZipStrict(tc.args, yes)
		if err != nil {
			t.Fatalf("%s: ZipStrict construct: %v", tc.name, err)
		}
		_, err = ListOf(objs(z))
		checkErr(t, tc.name, err, tc.want)
	}
	// Equal-length inputs drain cleanly under strict.
	z, err := ZipStrict(objs(newList(i(1), i(2)), newList(i(3), i(4))), yes)
	if err != nil {
		t.Fatalf("equal strict: %v", err)
	}
	lst, err := ListOf(objs(z))
	if err != nil {
		t.Fatalf("equal strict drain: %v", err)
	}
	if got := objects.Repr(lst); got != "[(1, 3), (2, 4)]" {
		t.Errorf("equal strict = %s, want [(1, 3), (2, 4)]", got)
	}
}
