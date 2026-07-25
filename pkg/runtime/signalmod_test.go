//go:build !windows

package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

func signalAttr(t *testing.T, name string) objects.Object {
	t.Helper()
	mo, err := ImportModule("_signal")
	if err != nil {
		t.Fatalf("import _signal: %v", err)
	}
	v, err := objects.LoadAttr(mo, name)
	if err != nil {
		t.Fatalf("_signal.%s: %v", name, err)
	}
	return v
}

func signalInt(t *testing.T, name string) int64 {
	t.Helper()
	v, ok := objects.AsInt(signalAttr(t, name))
	if !ok {
		t.Fatalf("_signal.%s is not an int", name)
	}
	return v
}

func TestSignalConstants(t *testing.T) {
	// SIG_DFL and SIG_IGN are the two sentinels, and the SIG* numbers follow the
	// standard POSIX assignment on every host this runs on.
	if v := signalInt(t, "SIG_DFL"); v != 0 {
		t.Errorf("SIG_DFL = %d, want 0", v)
	}
	if v := signalInt(t, "SIG_IGN"); v != 1 {
		t.Errorf("SIG_IGN = %d, want 1", v)
	}
	if v := signalInt(t, "SIGINT"); v != 2 {
		t.Errorf("SIGINT = %d, want 2", v)
	}
	if v := signalInt(t, "SIGTERM"); v != 15 {
		t.Errorf("SIGTERM = %d, want 15", v)
	}
	if v := signalInt(t, "NSIG"); v < 32 {
		t.Errorf("NSIG = %d, want at least 32", v)
	}
}

func TestSignalGetsignalDefaults(t *testing.T) {
	getsignal := signalAttr(t, "getsignal")
	sigint := signalAttr(t, "SIGINT")
	// SIGINT is seeded with default_int_handler, matching CPython's startup.
	h, err := objects.Call(getsignal, []objects.Object{sigint})
	if err != nil {
		t.Fatalf("getsignal(SIGINT): %v", err)
	}
	name, err := objects.LoadAttr(h, "__name__")
	if err != nil {
		t.Fatalf("handler __name__: %v", err)
	}
	if s, _ := objects.AsStr(name); s != "default_int_handler" {
		t.Errorf("getsignal(SIGINT) = %v, want default_int_handler", h)
	}
	// A signal with nothing installed reports SIG_DFL.
	sigterm := signalAttr(t, "SIGTERM")
	d, err := objects.Call(getsignal, []objects.Object{sigterm})
	if err != nil {
		t.Fatalf("getsignal(SIGTERM): %v", err)
	}
	if v, ok := objects.AsInt(d); !ok || v != sigDfl {
		t.Errorf("getsignal(SIGTERM) = %v, want SIG_DFL", d)
	}
}

func TestSignalInstallRoundTrip(t *testing.T) {
	sig := signalAttr(t, "signal")
	getsignal := signalAttr(t, "getsignal")
	sigusr1 := signalAttr(t, "SIGUSR1")
	handler := objects.NewFunc("h", -1, func(args []objects.Object) (objects.Object, error) {
		return objects.None, nil
	})
	// Installing a callable returns the previous handler and reads back by
	// identity.
	prev, err := objects.Call(sig, []objects.Object{sigusr1, handler})
	if err != nil {
		t.Fatalf("signal(SIGUSR1, handler): %v", err)
	}
	if v, ok := objects.AsInt(prev); !ok || v != sigDfl {
		t.Errorf("previous handler = %v, want SIG_DFL", prev)
	}
	cur, err := objects.Call(getsignal, []objects.Object{sigusr1})
	if err != nil {
		t.Fatalf("getsignal: %v", err)
	}
	if cur != handler {
		t.Errorf("getsignal(SIGUSR1) = %v, want the installed handler", cur)
	}
	// Restoring SIG_DFL hands the previous callable back and clears the slot.
	old, err := objects.Call(sig, []objects.Object{sigusr1, signalAttr(t, "SIG_DFL")})
	if err != nil {
		t.Fatalf("restore SIG_DFL: %v", err)
	}
	if old != handler {
		t.Errorf("restore returned %v, want the installed handler", old)
	}
}

func TestSignalErrors(t *testing.T) {
	sig := signalAttr(t, "signal")
	sigkill := signalAttr(t, "SIGKILL")
	// SIGKILL cannot take a handler.
	if _, err := objects.Call(sig, []objects.Object{sigkill, signalAttr(t, "SIG_IGN")}); err == nil {
		t.Errorf("signal(SIGKILL, SIG_IGN) did not raise")
	}
	// A non-callable, non-sentinel handler is rejected.
	if _, err := objects.Call(sig, []objects.Object{signalAttr(t, "SIGINT"), objects.NewInt(5)}); err == nil {
		t.Errorf("signal(SIGINT, 5) did not raise")
	}
}

func TestSignalStrsignalAndValid(t *testing.T) {
	strsignal := signalAttr(t, "strsignal")
	s, err := objects.Call(strsignal, []objects.Object{signalAttr(t, "SIGINT")})
	if err != nil {
		t.Fatalf("strsignal(SIGINT): %v", err)
	}
	if text, ok := objects.AsStr(s); !ok || len(text) < len("Interrupt") || text[:len("Interrupt")] != "Interrupt" {
		t.Errorf("strsignal(SIGINT) = %v, want an Interrupt description", s)
	}
	// An unknown number gives None, matching CPython.
	none, err := objects.Call(strsignal, []objects.Object{objects.NewInt(9999)})
	if err != nil {
		t.Fatalf("strsignal(9999): %v", err)
	}
	if none != objects.None {
		t.Errorf("strsignal(9999) = %v, want None", none)
	}
	// valid_signals is a set that names the common signals.
	valid := signalAttr(t, "valid_signals")
	vs, err := objects.Call(valid, nil)
	if err != nil {
		t.Fatalf("valid_signals(): %v", err)
	}
	has, err := objects.Contains(vs, signalAttr(t, "SIGINT"))
	if err != nil {
		t.Fatalf("SIGINT in valid_signals: %v", err)
	}
	if b, ok := objects.AsBool(has); !ok || !b {
		t.Errorf("SIGINT not in valid_signals")
	}
}
