package objects

import (
	"math/big"
	"strings"
	"testing"
)

// bigWithDigits builds the largest int with exactly n decimal digits, that is
// 10**n - 1 (n nines), the worst case for the bit-length fast path.
func bigWithDigits(n int) Object {
	b := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(n)), nil)
	b.Sub(b, big.NewInt(1))
	return NewIntFromBig(b)
}

// TestIntStrLimitBoundary pins the inclusive edge of the integer string
// conversion limit: a value with exactly 4300 digits renders, one more digit
// raises. The all-nines value is the case the bit-length fast path used to
// reject a digit early.
func TestIntStrLimitBoundary(t *testing.T) {
	atLimit := bigWithDigits(maxStrDigits) // 4300 nines
	s, err := StrE(atLimit)
	if err != nil {
		t.Fatalf("StrE at limit: unexpected error %v", err)
	}
	if len(s) != maxStrDigits {
		t.Fatalf("StrE at limit: got %d digits, want %d", len(s), maxStrDigits)
	}

	overLimit := bigWithDigits(maxStrDigits + 1) // 4301 nines
	if _, err := StrE(overLimit); err == nil {
		t.Fatalf("StrE over limit: want error, got none")
	} else if !strings.Contains(err.Error(), "Exceeds the limit") {
		t.Fatalf("StrE over limit: got %v, want conversion-limit error", err)
	}
}
