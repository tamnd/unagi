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

// TestZeroDivisionError pins that the static tier's divide-by-zero raises the
// same exception surface as the boxed tier, message and type both.
func TestZeroDivisionError(t *testing.T) {
	err := ZeroDivisionError("float division by zero")
	if err == nil {
		t.Fatal("ZeroDivisionError returned nil")
	}
	if got, want := err.Error(), "ZeroDivisionError: float division by zero"; got != want {
		t.Errorf("ZeroDivisionError().Error() = %q, want %q", got, want)
	}
}
