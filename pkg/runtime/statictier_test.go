package runtime

import (
	"math"
	"testing"
)

func TestAddInt64(t *testing.T) {
	cases := []struct {
		a, b, want int64
		ovf        bool
	}{
		{1, 2, 3, false},
		{-4, 1, -3, false},
		{math.MaxInt64, 0, math.MaxInt64, false},
		{math.MaxInt64, -1, math.MaxInt64 - 1, false},
		{math.MinInt64, math.MaxInt64, -1, false},
		{math.MaxInt64, 1, math.MinInt64, true},
		{math.MinInt64, -1, math.MaxInt64, true},
		{math.MaxInt64, math.MaxInt64, -2, true},
		{math.MinInt64, math.MinInt64, 0, true},
	}
	for _, c := range cases {
		got, ovf := AddInt64(c.a, c.b)
		if ovf != c.ovf || (!ovf && got != c.want) {
			t.Errorf("AddInt64(%d, %d) = (%d, %v), want (%d, %v)", c.a, c.b, got, ovf, c.want, c.ovf)
		}
	}
}

func TestSubInt64(t *testing.T) {
	cases := []struct {
		a, b, want int64
		ovf        bool
	}{
		{3, 1, 2, false},
		{1, 3, -2, false},
		{math.MaxInt64, math.MaxInt64, 0, false},
		{math.MinInt64, math.MinInt64, 0, false},
		{math.MinInt64, 1, math.MaxInt64, true},
		{math.MaxInt64, -1, math.MinInt64, true},
		{0, math.MinInt64, math.MinInt64, true},
	}
	for _, c := range cases {
		got, ovf := SubInt64(c.a, c.b)
		if ovf != c.ovf || (!ovf && got != c.want) {
			t.Errorf("SubInt64(%d, %d) = (%d, %v), want (%d, %v)", c.a, c.b, got, ovf, c.want, c.ovf)
		}
	}
}

func TestMulInt64(t *testing.T) {
	cases := []struct {
		a, b, want int64
		ovf        bool
	}{
		{0, math.MinInt64, 0, false},
		{math.MaxInt64, 0, 0, false},
		{6, 7, 42, false},
		{-6, 7, -42, false},
		{math.MinInt64, 1, math.MinInt64, false},
		{math.MaxInt64, 1, math.MaxInt64, false},
		{math.MaxInt64, 2, 0, true},
		{math.MinInt64, -1, 0, true},
		{-1, math.MinInt64, 0, true},
		{math.MinInt64, 2, 0, true},
	}
	for _, c := range cases {
		got, ovf := MulInt64(c.a, c.b)
		if ovf != c.ovf || (!ovf && got != c.want) {
			t.Errorf("MulInt64(%d, %d) = (%d, %v), want (%d, %v)", c.a, c.b, got, ovf, c.want, c.ovf)
		}
	}
}

func TestFloorDivInt64(t *testing.T) {
	cases := []struct {
		a, b, want int64
		ovf        bool
	}{
		// Same-sign operands agree with Go's truncating divide.
		{7, 2, 3, false},
		{6, 3, 2, false},
		{-7, -2, 3, false},
		{-6, -3, 2, false},
		// Mixed-sign inexact division floors toward negative infinity, one below
		// what Go's truncation gives.
		{-7, 2, -4, false},
		{7, -2, -4, false},
		// Mixed-sign exact division needs no correction.
		{-6, 3, -2, false},
		{6, -3, -2, false},
		// Boundaries that must not falsely flag: MinInt64 divided by anything but -1.
		{math.MinInt64, 1, math.MinInt64, false},
		{math.MinInt64, 2, math.MinInt64 / 2, false},
		{math.MaxInt64, 1, math.MaxInt64, false},
		// The one overflow: MinInt64 // -1 is 2**63, one past int64.
		{math.MinInt64, -1, 0, true},
	}
	for _, c := range cases {
		got, ovf := FloorDivInt64(c.a, c.b)
		if ovf != c.ovf || (!ovf && got != c.want) {
			t.Errorf("FloorDivInt64(%d, %d) = (%d, %v), want (%d, %v)", c.a, c.b, got, ovf, c.want, c.ovf)
		}
	}
}

func TestFloorModInt64(t *testing.T) {
	cases := []struct {
		a, b, want int64
	}{
		// Same-sign operands agree with Go's truncating remainder.
		{7, 3, 1},
		{6, 3, 0},
		{-7, -3, -1},
		{-6, -3, 0},
		// Mixed-sign nonzero remainders carry the divisor's sign, one correction away
		// from Go's dividend-signed remainder.
		{-7, 3, 2},
		{7, -3, -2},
		// Mixed-sign exact division leaves a zero remainder, no correction.
		{-6, 3, 0},
		{6, -3, 0},
		// The boundary Go defines as zero rather than trapping, which is Python's answer.
		{math.MinInt64, -1, 0},
		{math.MinInt64, 1, 0},
	}
	for _, c := range cases {
		if got := FloorModInt64(c.a, c.b); got != c.want {
			t.Errorf("FloorModInt64(%d, %d) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

// TestZeroDivisionError pins that the static tier's divide-by-zero raises the
// same exception surface as the boxed tier, message and type both.
func TestZeroDivisionError(t *testing.T) {
	err := ZeroDivisionError("division by zero")
	if err == nil {
		t.Fatal("ZeroDivisionError returned nil")
	}
	if got, want := err.Error(), "ZeroDivisionError: division by zero"; got != want {
		t.Errorf("ZeroDivisionError().Error() = %q, want %q", got, want)
	}
}
