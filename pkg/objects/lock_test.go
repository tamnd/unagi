package objects

import "testing"

// TestLockAcquireReleaseLocked walks the plain Lock: a fresh lock is free, an
// acquire takes it, a non-blocking acquire on a held lock fails, and a release
// frees it again.
func TestLockAcquireReleaseLocked(t *testing.T) {
	l := NewLock()
	if l.locked() {
		t.Fatal("fresh lock reports locked")
	}
	if !l.acquire(true, -1) {
		t.Fatal("acquire on a free lock returned false")
	}
	if !l.locked() {
		t.Fatal("held lock reports unlocked")
	}
	if l.acquire(false, 0) {
		t.Fatal("non-blocking acquire on a held lock returned true")
	}
	if err := l.release(); err != nil {
		t.Fatalf("release of a held lock: %v", err)
	}
	if l.locked() {
		t.Fatal("released lock still reports locked")
	}
}

// TestLockReleaseUnlocked is the RuntimeError CPython raises for releasing a lock
// that is not held.
func TestLockReleaseUnlocked(t *testing.T) {
	l := NewLock()
	err := l.release()
	if err == nil || Str(err.(*Exception)) != "release unlocked lock" {
		t.Fatalf("release of a free lock error = %v, want the RuntimeError", err)
	}
}

// TestParseAcquireErrors covers the two argument rules: a non-blocking call may
// not carry a timeout, and a timeout other than -1 must be non-negative.
func TestParseAcquireErrors(t *testing.T) {
	if _, _, err := parseAcquire("acquire", []Object{False, NewInt(1)}, nil, nil); err == nil ||
		Str(err.(*Exception)) != "can't specify a timeout for a non-blocking call" {
		t.Fatalf("blocking=False with timeout error = %v, want the ValueError", err)
	}
	if _, _, err := parseAcquire("acquire", []Object{True, NewInt(-5)}, nil, nil); err == nil ||
		Str(err.(*Exception)) != "timeout value must be a non-negative number" {
		t.Fatalf("negative timeout error = %v, want the ValueError", err)
	}
	// -1 is the block-forever sentinel and is allowed.
	if _, d, err := parseAcquire("acquire", []Object{True, NewInt(-1)}, nil, nil); err != nil || d != -1 {
		t.Fatalf("timeout=-1 = (%v, %v), want (-1, nil)", d, err)
	}
}

// TestRLockReentrantAndOwnership drives the reentrant lock through two distinct
// thread states: the owner acquires it twice without blocking, a non-owner
// cannot release it, and only the matching number of releases frees it.
func TestRLockReentrantAndOwnership(t *testing.T) {
	owner := NewThread("owner", false)
	other := NewThread("other", false)
	r := NewRLock()

	if _, err := rlockMethodT(owner, r, "acquire", nil); err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	if _, err := rlockMethodT(owner, r, "acquire", nil); err != nil {
		t.Fatalf("reentrant acquire: %v", err)
	}
	owned, _ := rlockMethodT(owner, r, "_is_owned", nil)
	if !Truth(owned) {
		t.Fatal("_is_owned() false for the holding thread")
	}
	notOwned, _ := rlockMethodT(other, r, "_is_owned", nil)
	if Truth(notOwned) {
		t.Fatal("_is_owned() true for a thread that does not hold the lock")
	}

	// A release by a thread that does not own the lock is the RuntimeError.
	if _, err := rlockMethodT(other, r, "release", nil); err == nil ||
		Str(err.(*Exception)) != "cannot release un-acquired lock" {
		t.Fatalf("non-owner release error = %v, want the RuntimeError", err)
	}

	// Two acquires need two releases before the lock is free.
	if _, err := rlockMethodT(owner, r, "release", nil); err != nil {
		t.Fatalf("first release: %v", err)
	}
	if !r.inner.locked() {
		t.Fatal("lock freed after one of two releases")
	}
	if _, err := rlockMethodT(owner, r, "release", nil); err != nil {
		t.Fatalf("second release: %v", err)
	}
	if r.inner.locked() {
		t.Fatal("lock still held after the matching releases")
	}
}
