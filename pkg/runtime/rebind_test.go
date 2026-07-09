package runtime

import (
	"math/big"
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// TestRebindIntExactBindingIsVersionOne checks the world-age shadow update for an
// int global: an exact int fitting int64 gives the native value and version 1, so
// the static form's entry guard passes and its fast path reads the shadow. A bool,
// a float, a spilled big int, an unbound (nil) binding, and a non-scalar object
// all give version 2 (or the unbound 0 the counter starts at is handled by the
// guard), so the read routes to the boxed twin instead of a stale or coerced
// shadow.
func TestRebindIntExactBindingIsVersionOne(t *testing.T) {
	if v, ver := RebindInt(objects.NewInt(7)); v != 7 || ver != 1 {
		t.Errorf("RebindInt(7) = (%d, %d), want (7, 1)", v, ver)
	}
	// A bool is exactly bool, not int: True prints as True, so an int global rebound
	// to True must deopt rather than read 1.
	if _, ver := RebindInt(objects.NewBool(true)); ver != 2 {
		t.Errorf("RebindInt(True) version = %d, want 2 (a bool is not an int shadow)", ver)
	}
	if _, ver := RebindInt(objects.NewFloat(7)); ver != 2 {
		t.Errorf("RebindInt(7.0) version = %d, want 2", ver)
	}
	big := objects.NewIntFromBig(new(big.Int).Lsh(big.NewInt(1), 70))
	if _, ver := RebindInt(big); ver != 2 {
		t.Errorf("RebindInt(spilled big int) version = %d, want 2", ver)
	}
	if _, ver := RebindInt(nil); ver != 2 {
		t.Errorf("RebindInt(nil) version = %d, want 2", ver)
	}
}

// TestRebindFloatBoolStrExactness checks the remaining scalar rebinds are exact on
// the dynamic type, so a coercible-but-distinct value deopts instead of reading a
// coerced shadow.
func TestRebindFloatBoolStrExactness(t *testing.T) {
	if v, ver := RebindFloat(objects.NewFloat(1.5)); v != 1.5 || ver != 1 {
		t.Errorf("RebindFloat(1.5) = (%v, %d), want (1.5, 1)", v, ver)
	}
	if _, ver := RebindFloat(objects.NewInt(2)); ver != 2 {
		t.Errorf("RebindFloat(int 2) version = %d, want 2", ver)
	}
	if v, ver := RebindBool(objects.NewBool(true)); v != true || ver != 1 {
		t.Errorf("RebindBool(True) = (%v, %d), want (true, 1)", v, ver)
	}
	if _, ver := RebindBool(objects.NewInt(1)); ver != 2 {
		t.Errorf("RebindBool(int 1) version = %d, want 2", ver)
	}
	if v, ver := RebindStr(objects.NewStr("hi")); v != "hi" || ver != 1 {
		t.Errorf("RebindStr(hi) = (%q, %d), want (hi, 1)", v, ver)
	}
	if _, ver := RebindStr(objects.NewInt(1)); ver != 2 {
		t.Errorf("RebindStr(int 1) version = %d, want 2", ver)
	}
}
