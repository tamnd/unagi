package objects

import "testing"

// TestMemoryviewHexSep checks that memoryview.hex honors the separator and
// bytes-per-group arguments the way bytes.hex does, grouping the buffer bytes.
func TestMemoryviewHexSep(t *testing.T) {
	mv, err := NewMemoryView(NewBytes([]byte("hello")))
	if err != nil {
		t.Fatalf("NewMemoryView: %v", err)
	}
	r, err := CallMethod(mv, "hex", []Object{NewStr("-")})
	if err != nil {
		t.Fatalf("hex('-'): %v", err)
	}
	if s, _ := r.(*strObject); s == nil || s.v != "68-65-6c-6c-6f" {
		t.Fatalf("hex('-') = %v, want 68-65-6c-6c-6f", r)
	}
	mv2, err := NewMemoryView(NewBytes([]byte("abcdef")))
	if err != nil {
		t.Fatalf("NewMemoryView: %v", err)
	}
	r, err = CallMethod(mv2, "hex", []Object{NewStr("_"), NewInt(-2)})
	if err != nil {
		t.Fatalf("hex('_', -2): %v", err)
	}
	if s, _ := r.(*strObject); s == nil || s.v != "6162_6364_6566" {
		t.Fatalf("hex('_', -2) = %v, want 6162_6364_6566", r)
	}
	// A multi-character separator and too many arguments raise the same errors
	// bytes.hex spells, the arity message matching CPython's slot wording.
	if _, err := CallMethod(mv, "hex", []Object{NewStr("--")}); err == nil {
		t.Fatal("hex('--') did not raise")
	}
	_, err = CallMethod(mv, "hex", []Object{NewStr("-"), NewInt(2), NewInt(3)})
	checkErr(t, "hex too many", err, "TypeError: hex() takes at most 2 arguments (3 given)")
}
