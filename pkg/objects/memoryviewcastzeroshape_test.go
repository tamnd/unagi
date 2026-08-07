package objects

import (
	"strings"
	"testing"
)

// castErr reshapes an empty or sized view and returns the error message, failing
// if the cast unexpectedly succeeds.
func castErr(t *testing.T, src Object, format string, shape []Object) string {
	t.Helper()
	mv, err := NewMemoryView(src)
	if err != nil {
		t.Fatalf("memoryview: %v", err)
	}
	_, err = mvCast(mv.(*memoryviewObject), []Object{NewStr(format), NewList(shape)})
	if err == nil {
		t.Fatalf("cast succeeded; want an error")
	}
	return err.Error()
}

// TestMemoryviewCastEmptySourceReshapeRaisesZeros checks that reshaping a
// zero-length view reports the cannot-cast-zeros TypeError CPython raises, ahead
// of the per-element shape checks, so even a shape with a bad element still shows
// the zero-view error first.
func TestMemoryviewCastEmptySourceReshapeRaisesZeros(t *testing.T) {
	empty := NewBytes(nil)
	want := "cannot cast view with zeros in shape or strides"
	for _, shape := range [][]Object{
		{NewInt(0)},
		{NewInt(1)},
		{NewInt(0), NewInt(1)},
		{NewInt(1), NewInt(0)},
		{},
		{NewStr("x")},
		{NewInt(-1)},
	} {
		got := castErr(t, empty, "B", shape)
		if !strings.Contains(got, want) {
			t.Fatalf("cast empty shape=%v = %q; want it to mention %q", shape, got, want)
		}
	}
}

// TestMemoryviewCastSizedSourceZeroDimRaisesValue guards the boundary: a sized
// view keeps CPython's per-element wording, so a zero or negative dimension is
// the elements-must-be-positive ValueError rather than the zero-view TypeError.
func TestMemoryviewCastSizedSourceZeroDimRaisesValue(t *testing.T) {
	sized := NewBytes(make([]byte, 6))
	for _, shape := range [][]Object{
		{NewInt(0), NewInt(3)},
		{NewInt(2), NewInt(0)},
		{NewInt(-1), NewInt(3)},
	} {
		got := castErr(t, sized, "B", shape)
		if !strings.Contains(got, "elements of shape must be integers > 0") {
			t.Fatalf("cast sized shape=%v = %q; want the positive-elements ValueError", shape, got)
		}
	}
	// A non-integer element on a sized source is still the integers TypeError.
	got := castErr(t, sized, "B", []Object{NewInt(2), NewStr("x")})
	if !strings.Contains(got, "elements of shape must be integers") {
		t.Fatalf("cast sized shape=[2,'x'] = %q; want the integers TypeError", got)
	}
}

// TestMemoryviewCastEmptySourceNoShapeSucceeds checks the guard is scoped to the
// reshape path: casting an empty view without a shape still succeeds and yields
// an empty view under the new format.
func TestMemoryviewCastEmptySourceNoShapeSucceeds(t *testing.T) {
	mv, err := NewMemoryView(NewBytes(nil))
	if err != nil {
		t.Fatalf("memoryview: %v", err)
	}
	out, err := mvCast(mv.(*memoryviewObject), []Object{NewStr("I")})
	if err != nil {
		t.Fatalf("cast('I') on empty view: %v", err)
	}
	if got := out.(*memoryviewObject).length; got != 0 {
		t.Fatalf("empty cast length = %d; want 0", got)
	}
}
